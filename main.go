package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gocolly/colly/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PageData struct {
	URL         string `bson:"url" json:"url"`
	Title       string `bson:"title" json:"title"`
	Description string `bson:"description" json:"description"`
}

func main() {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := "mongodb://localhost:27017"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB connect error: %v", err)
	}

	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	db := client.Database("webcrawler")
	collection := db.Collection("pages_data")

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "url", Value: 1}},
		Options: options.Index().SetUnique(true).SetBackground(true),
	}
	if _, err := collection.Indexes().CreateOne(context.Background(), indexModel); err != nil {
		log.Fatalf("Failed creating index: %v", err)
	}

	var results []PageData

	c := colly.NewCollector(
		colly.AllowedDomains("blinkeet-rho.vercel.app", "www.blinkeet-rho.vercel.app"),
		colly.MaxDepth(2),
	)
	c.Async = true

	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 4,
		RandomDelay: 1 * time.Second,
	}); err != nil {
		log.Fatalf("Failed to set limit: %v", err)
	}

	c.OnHTML("head", func(e *colly.HTMLElement) {
		page := PageData{
			URL: e.Request.URL.String(),
		}
		page.Title = e.DOM.Find("title").Text()
		page.Description, _ = e.DOM.Find(`meta[name="description"]`).Attr("content")

		results = append(results, page)

		// Insert into MongoDB — duplicates handled by unique index
		_, err := collection.InsertOne(context.Background(), page)
		if err != nil {

			// If duplicate key (URL already present), ignore; otherwise log
			var writeEx mongo.WriteException
			if errors.As(err, &writeEx) {

				duplicate := false
				for _, we := range writeEx.WriteErrors {
					if we.Code == 11000 {
						duplicate = true
						break
					}
				}
				if duplicate {
					fmt.Println("Duplicate (skipped):", page.URL)
					return
				}
			}

			log.Printf("Mongo insert error for %s: %v\n", page.URL, err)
			return
		}
		fmt.Println("Inserted:", page.URL)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if link == "" {
			return
		}

		c.Visit(link)
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting:", r.URL.String())
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Request URL: %s failed: %v (status: %d)\n", r.Request.URL, err, r.StatusCode)
	})

	startURL := "https://blinkeet-rho.vercel.app"
	fmt.Println("Starting crawl on:", startURL)
	if err := c.Visit(startURL); err != nil {
		log.Fatalf("Visit error: %v", err)
	}

	c.Wait()

	file, err := os.Create("results.json")
	if err != nil {
		log.Fatalf("Could not create results.json: %v", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		log.Fatalf("Error writing JSON: %v", err)
	}

	fmt.Println("Crawl finished. results.json written and data stored in MongoDB (webcrawler.pages_data).")
}
