package frilanse

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"code.google.com/p/cascadia"
	"code.google.com/p/go.net/html"
	"code.google.com/p/go.net/html/charset"
)

var jobs []*Job
var jobsMu sync.Mutex

func Start(addr string) {
	ch := make(chan *Job)

	go serveHTTP(addr)

	go Amino(ch)
	go Flexer(ch)
	go KonsulenterDK(ch)
	go SCR(ch)
	go RightPeople(ch)
	go Ahoc(ch)
	// TODO - http://www.ework.dk/currentProjects.cfm?COUNTRY_ID=440
	// TODO - http://www.twins.net/consulting/for-freelancere/opgaver.aspx
	// TODO - http://www.accuro.dk/JOB/freelanceopgaver.htm
	// TODO - http://www.jobzonen.dk/jobsrss/?Jobtype=39&countrycode=DK

	for job := range ch {
		jobsMu.Lock()
		jobs = append(jobs, job)

		if job.Date.After(time.Now().Add(time.Hour * -1)) {
			log.Println("NEW", job.Title)
		}

		jobsMu.Unlock()
	}
}

func removeJob(job *Job) {
	jobsMu.Lock()
	defer jobsMu.Unlock()

	log.Println("REMOVE", job.Title)

	for i, _ := range jobs {
		if jobs[i].Link.String() == job.Link.String() {
			jobs = append(jobs[:i], jobs[i+1:]...)
			break
		}
	}
}

type Job struct {
	Title string
	Link  *url.URL
	Date  time.Time
}

func (j Job) String() string { return j.Title }

func WithGet(url string, interval time.Duration, fn func(*http.Response) error) {
	for ; ; time.Sleep(interval) {
		r, err := http.Get(url)
		if err != nil {
			log.Printf("%q: get: %s", url, err)
			continue
		}
		defer r.Body.Close()

		if err := fn(r); err != nil {
			log.Printf("%q: process: %s", err)
		}
	}
}

func WithDoc(url string, interval time.Duration, fn func(*html.Node) error) {
	WithGet(url, interval, func(r *http.Response) error {
		rd, err := charset.NewReader(r.Body, "")
		if err != nil {
			return fmt.Errorf("encoding: %s", err)
		}

		doc, err := html.Parse(rd)
		if err != nil {
			return fmt.Errorf("parse: %s", err)
		}

		if err := fn(doc); err != nil {
			return fmt.Errorf("process: %s", err)
		}

		return nil
	})
}

func Amino(jobs chan *Job) {
	throttle := time.NewTicker(time.Second * 5)

	for item := range NewFeedReader("http://www.amino.dk/freelancer/handlers/feeds.ashx?feed=1") {
		link, err := url.Parse(item.Link)
		if err != nil {
			log.Printf("amino.dk: %q: could not parse URL: %s", item.Link, err)
			continue
		}

		date, err := time.Parse("2006-01-02T15:04:05Z07:00", item.Updated)
		if err != nil {
			log.Printf("amino.dk: could not parse Date: %s", err)
			continue
		}

		job := &Job{Title: html.UnescapeString(item.Title), Link: link, Date: date}

		go func(job *Job) {
			<-throttle.C

			if ok, err := AminoIsValid(job); err != nil || !ok {
				return
			}

			go func(job *Job) {
				for {
					time.Sleep(time.Hour * 1)

					if ok, err := AminoIsValid(job); err == nil && !ok {
						break
					}
				}
				removeJob(job)
			}(job)

			jobs <- job
		}(job)
	}
}

func AminoIsValid(job *Job) (bool, error) {
	r, err := http.Get(job.Link.String())
	if err != nil {
		return false, fmt.Errorf("could not GET page: %s", err)
	}
	defer r.Body.Close()

	doc, err := html.Parse(r.Body)
	if err != nil {
		return false, fmt.Errorf("could not parse HTML: %s", err)
	}

	for _, n := range cascadia.MustCompile(`span#ctl00_content_lblJobName`).MatchAll(doc) {
		if n.FirstChild != nil && n.FirstChild.Data == "Opgaven eksisterer ikke" {
			return false, nil
		}
	}

	ps := cascadia.MustCompile(`div#ctl00_content_pnlJobDetails2 > p.detail`).MatchAll(doc)
	if len(ps) == 3 {
		var now = time.Now()
		if t, err := time.Parse("02-01-2006", ps[2].FirstChild.Data); err != nil || t.Year() < now.Year() || (t.Year() == now.Year() && t.Month() < now.Month()) || (t.Year() == now.Year() && t.Month() == now.Month() && t.Day() < now.Day()) {
			return false, nil
		} else {
			return true, nil
		}
	}

	return false, nil
}

func RightPeople(jobs chan *Job) {
	var (
		seen     = make(map[string]bool)
		firstRun = true
	)

	WithDoc("http://rightpeople.dk/consultants/be-right-join-us.html", time.Minute*5, func(doc *html.Node) error {
		for _, a := range cascadia.MustCompile(`a`).MatchAll(doc) {
			var href string
			for _, a := range a.Attr {
				if a.Key == "href" {
					href = html.UnescapeString(a.Val)
				}
			}
			var link *url.URL
			if !strings.HasPrefix(href, "http://www.rightpeople.dk/component/option,com_ckeditor/lang,da/plugin,linkBrowser/task,plugin/?option=com_content&view=article") || !strings.HasPrefix(href, "http://www.rightpeople.dk/component/option,com_ckeditor/lang,da/plugin,linkBrowser/task,plugin/?option=com_content&view=article") {
				continue
			} else if _, ok := seen[href]; ok {
				continue
			} else if u, err := url.Parse(href); err != nil {
				return fmt.Errorf("Parse URL: %s", err)
			} else {
				seen[href] = true
				link = u
			}

			var date time.Time
			if !firstRun {
				date = time.Now()
			}

			job := &Job{Title: a.FirstChild.Data, Link: link, Date: date}

			if ms := regexp.MustCompile(`(\d+-\d+-\d+)`).FindStringSubmatch(a.FirstChild.Data); len(ms) > 1 {
				date, err := time.Parse("2-1-2006", ms[len(ms)-1])
				if err != nil {
					return fmt.Errorf("could not parse date: %s", err)
				} else {
					job.Date = date
				}
			}

			jobs <- job
		}

		firstRun = false

		return nil
	})
}

func Flexer(jobs chan *Job) {
	var (
		seen     = make(map[string]bool)
		ref, _   = url.Parse("http://www.flexer.dk/")
		throttle = time.NewTicker(time.Second * 5)
	)

	WithDoc("http://www.flexer.dk/tasks", time.Minute*5, func(doc *html.Node) error {
		for _, tr := range cascadia.MustCompile(`tr.hand`).MatchAll(doc) {
			as := cascadia.MustCompile(`td.task-name > a`).MatchAll(tr)
			if len(as) != 1 {
				return fmt.Errorf("could not get <a>")
			}
			var title, href string
			for _, a := range as[0].Attr {
				switch a.Key {
				case "title":
					title = a.Val
				case "href":
					href = a.Val
				}
			}
			var link *url.URL
			if u, err := ref.Parse(href); err != nil {
				return fmt.Errorf("parse href: %s", err)
			} else {
				link = u
			}

			tds := cascadia.MustCompile(`td.task-date`).MatchAll(tr)
			if len(tds) != 1 || tds[0].FirstChild == nil {
				return fmt.Errorf("could not get <td class='task-date'>")
			}
			var date time.Time
			if text := tds[0].FirstChild.Data; text == "I dag" {
				date = time.Now()
			} else if text == "I går" {
				date = time.Now().AddDate(0, 0, -1)
			} else if d, err := time.Parse("02-01-2006", text); err != nil {
				return fmt.Errorf("parse date: %s", err)
			} else {
				date = d
			}

			if _, ok := seen[href]; ok {
				continue
			} else {
				seen[href] = true
			}

			go func(job *Job) {
				<-throttle.C

				if ok, err := FlexerIsValid(job); err == nil && !ok {
					return
				}

				go func(job *Job) {
					for {
						time.Sleep(time.Hour * 1)
						if ok, err := FlexerIsValid(job); err == nil && !ok {
							break
						}
					}

					removeJob(job)
				}(job)

				jobs <- job
			}(&Job{Title: title, Link: link, Date: date})
		}

		return nil
	})
}

func FlexerIsValid(job *Job) (bool, error) {
	r, err := http.Get(job.Link.String())
	if err != nil {
		return false, err
	}
	defer r.Body.Close()

	doc, err := html.Parse(r.Body)
	if err != nil {
		return false, err
	}

	for _, p := range cascadia.MustCompile(`p`).MatchAll(doc) {
		if p.FirstChild != nil && p.FirstChild.Data == "Opgaven findes ikke" {
			return false, nil
		}
	}

	for _, n := range cascadia.MustCompile(`div#content > table.listing > tbody > tr`).MatchAll(doc) {
		if n.FirstChild.NextSibling.FirstChild.Data != "Deadline" {
			continue
		}

		var now = time.Now()
		if t, err := time.Parse("2006-01-02", n.FirstChild.NextSibling.NextSibling.NextSibling.FirstChild.Data); err != nil {
			return false, err
		} else if t.Year() > now.Year() || (t.Year() == now.Year() && t.Month() > now.Month()) || (t.Year() == now.Year() && t.Month() == now.Month() && t.Day() >= now.Day()) {
			return true, nil
		}
	}

	return false, nil
}

func Ahoc(jobs chan *Job) {
	seen := make(map[string]bool)
	ref, _ := url.Parse("http://ahoc.dk/page.aspx?id=218")
	firstRun := true

	WithDoc("http://ahoc.dk/page.aspx?id=218", time.Minute*5, func(doc *html.Node) error {
		for _, a := range cascadia.MustCompile(`div#submenu > ul:first-child > li > a`).MatchAll(doc) {
			var href string
			for _, a := range a.Attr {
				if a.Key == "href" {
					href = a.Val
				}
			}

			var link *url.URL
			if _, ok := seen[href]; ok {
				continue
			} else if u, err := ref.Parse(href); err != nil {
				return fmt.Errorf("Parse URL: %s", err)
			} else {
				seen[href] = true
				link = u
			}

			var date time.Time
			if !firstRun {
				date = time.Now()
			}

			jobs <- &Job{Title: a.FirstChild.Data, Link: link, Date: date}
		}

		firstRun = false

		return nil
	})
}

func KonsulenterDK(jobs chan *Job) {
	for item := range NewFeedReader("http://www.konsulenter.dk/opgave/rss/") {
		link, err := url.Parse(item.Link)
		if err != nil {
			log.Printf("konsulenter.dk: %q: could not parse URL: %s", item.Link, err)
			continue
		}

		date, err := time.Parse(time.RFC1123, item.PubDate)
		if err != nil {
			log.Printf("konsulenter.dk: could not parse Date: %s", err)
			continue
		}

		jobs <- &Job{
			Title: item.Title,
			Link:  link,
			Date:  date,
		}
	}
}

func SCR(jobs chan *Job) {
	var (
		seen     = make(map[string]bool)
		ref, _   = url.Parse("http://www.scr.dk/Default.aspx?ID=21&M=News&PID=27&NewsID=1317")
		firstRun = true
	)

	re := regexp.MustCompile(`new NewsItem\(\d+, \d+, \d+, \d+, '[^']+', '([^']+)', \d+, '<a href="([^"]+)">L`)

	WithGet("http://www.scr.dk/Default.aspx?ID=21&M=News&PID=27&NewsID=1317", time.Minute*5, func(r *http.Response) error {
		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("read Body: %s", err)
		}

		for _, ms := range re.FindAllSubmatch(bytes, -1) {
			if len(ms) != 3 {
				return fmt.Errorf("Submatch len: %d", len(ms))
			}
			var href = html.UnescapeString(string(ms[2]))

			if _, ok := seen[href]; ok {
				continue
			} else {
				seen[href] = true
			}

			var link *url.URL
			if u, err := ref.Parse(href); err != nil {
				return fmt.Errorf("Parse URL: %s", err)
			} else {
				link = u
			}

			var date time.Time
			if !firstRun {
				date = time.Now()
			}

			jobs <- &Job{Title: string(ms[1]), Link: link, Date: date}
		}

		firstRun = false

		return nil
	})
}
