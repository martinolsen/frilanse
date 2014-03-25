package frilanse

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
)

func serveHTTP(addr string) {
	http.HandleFunc("/", indexHandler)

	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}

type JobsDateSorter []*Job

func (js JobsDateSorter) Len() int           { return len(js) }
func (js JobsDateSorter) Less(i, j int) bool { return js[i].Date.Before(js[j].Date) }
func (js JobsDateSorter) Swap(i, j int)      { js[i], js[j] = js[j], js[i] }

func indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "<!DOCTYPE html>")
	fmt.Fprintln(w, "<html><head><style>")
	fmt.Fprintln(w, `ul {margin: 1.3em; border: 0}`)
	fmt.Fprintln(w, `li {list-style: none; padding: 0.7em 1.2em 0.7em 1.2em; background-size: 0.9em}`)

	fmt.Fprintln(w, "</style></head><body>")

	var sortedJobs = make([]*Job, len(jobs))
	copy(sortedJobs, jobs)

	sort.Sort(sort.Reverse(JobsDateSorter(sortedJobs)))

	fmt.Fprintln(w, "<ul>")
	for _, job := range sortedJobs {
		var favicon string
		if regexp.MustCompile(`^https?://[^/]*ahoc.dk/`).MatchString(job.Link.String()) {
			// ignore - they have no favicon!
		} else if regexp.MustCompile(`^https?://[^/]*rightpeople.dk/`).MatchString(job.Link.String()) {
			favicon = "http://www.rightpeople.dk/templates/rightpeople/favicon.ico"
		} else if regexp.MustCompile(`^https?://[^/]*scr.dk/`).MatchString(job.Link.String()) {
			favicon = "http://www.scr.dk/Files/Favikon.ico"
		} else if m := regexp.MustCompile(`^https?://[^/]+`).FindString(job.Link.String()); m != "" {
			favicon = m + "/favicon.ico"
		}

		if favicon != "" {
			fmt.Fprintf(w, `<li style="background: url('%s') no-repeat left center">`, favicon)
		} else { fmt.Fprint(w, "<li>") }

		fmt.Fprintf(w, "<a href=%q>%s</a>\n", job.Link.String(), job.Title)
	}
	fmt.Fprintln(w, "</ul>")

	fmt.Fprintln(w, "</body></html>")
}
