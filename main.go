package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gocolly/colly/v2"
)

func main() {
	c := colly.NewCollector(
		colly.AllowedDomains("golang.org", "go.dev"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		RandomDelay: 2 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting:", r.URL.String())
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	c.OnHTML("title", func(e *colly.HTMLElement) {
		fmt.Println("Page Title:", e.Text)
	})

	c.OnHTML("meta[name=description]", func(e *colly.HTMLElement) {
		desc := e.Attr("content")
		fmt.Println("Meta Description:", desc)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if link != "" {
			c.Visit(link)
		}
	})

	err := c.Visit("https://golang.org")
	if err != nil {
		log.Fatal(err)
	}

	c.Wait()
}
