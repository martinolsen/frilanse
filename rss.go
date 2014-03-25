package frilanse

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"fmt"
	"time"
)

type FeedReader chan *RSSItem

type RSS struct {
	XMLName xml.Name   `xml:"rss"`
	Items   []*RSSItem `xml:"channel>item"`
}

type RSSItem struct {
	Id          string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`

	Updated string `xml:"http://www.w3.org/2005/Atom updated"`
}

func NewFeedReader(url string) FeedReader {
	seen := make(map[string]bool)
	items := make(chan *RSSItem)

	go func() {
		WithGet(url, time.Minute * 5, func(r *http.Response) error {
			var rss = &RSS{}
			if bytes, err := ioutil.ReadAll(r.Body); err != nil {
				return fmt.Errorf("read Body: %s", err)
			} else if err := xml.Unmarshal(bytes, rss); err != nil {
				return fmt.Errorf("unmarshal XML: %s", err)
			}

			for _, item := range rss.Items {
				if _, ok := seen[item.Id]; ok {
					continue
				}
				seen[item.Id] = true
				items <- item
			}

			return nil
		})

		close(items)
	}()

	return items
}
