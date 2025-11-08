package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gocolly/colly/v2"
)

type PageData struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func main() {
	var results []PageData

	c := colly.NewCollector(
		colly.AllowedDomains("golang.org", "go.dev"), // Only these domains
		colly.MaxDepth(2), // Crawl depth
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
		log.Println("Request failed:", r.Request.URL, err)
	})

	c.OnHTML("head", func(e *colly.HTMLElement) {
		page := PageData{
			URL: e.Request.URL.String(),
		}

		page.Title = e.DOM.Find("title").Text()
		page.Description, _ = e.DOM.Find(`meta[name="description"]`).Attr("content")

		results = append(results, page)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if link != "" {
			c.Visit(link)
		}
	})

	startURL := "https://golang.org"
	fmt.Println("Starting crawl on:", startURL)
	err := c.Visit(startURL)
	if err != nil {
		log.Fatal(err)
	}

	c.Wait()

	file, err := os.Create("results.json")
	if err != nil {
		log.Fatal("Could not create file:", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		log.Fatal("Error writing JSON:", err)
	}

	fmt.Println("\n Crawl finished! Data saved to results.json")
}
