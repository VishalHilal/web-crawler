package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PageData struct {
	URL         string    `bson:"url" json:"url"`
	Title       string    `bson:"title" json:"title"`
	Description string    `bson:"description" json:"description"`
	Images      []string  `bson:"images" json:"images"`
	Links       []string  `bson:"links" json:"links"`
	Timestamp   time.Time `bson:"timestamp" json:"timestamp"`
}

func main() {
	// MongoDB connection setup
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("MongoDB connection failed:", err)
	}
	defer client.Disconnect(context.TODO())

	collection := client.Database("crawlerDB").Collection("pages")

	// Colly setup
	c := colly.NewCollector(
		colly.AllowedDomains("blinkeet-rho.vercel.app", "golang.org", "go.dev"),

		colly.MaxDepth(2),
		colly.Async(true),
		colly.CacheDir("./cache"),
	)

	// Set rate limit (avoid bans)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 4,
		RandomDelay: 2 * time.Second,
	})

	// Regex to skip unwanted URLs
	skipPattern := regexp.MustCompile(`(login|signup|cart|logout)`)

	// Handler for main content (extract data)
	c.OnHTML("head", func(e *colly.HTMLElement) {
		page := PageData{
			URL:       e.Request.URL.String(),
			Timestamp: time.Now(),
		}

		page.Title = e.DOM.Find("title").Text()
		page.Description, _ = e.DOM.Find(`meta[name="description"]`).Attr("content")

		e.DOM.Find("img").Each(func(_ int, img *goquery.Selection) {
			src, _ := img.Attr("src")
			if src != "" {
				page.Images = append(page.Images, e.Request.AbsoluteURL(src))
			}
		})

		e.DOM.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
			href, _ := link.Attr("href")
			absURL := e.Request.AbsoluteURL(href)
			if absURL != "" && !skipPattern.MatchString(absURL) {
				page.Links = append(page.Links, absURL)
			}
		})

		// Save to MongoDB (Upsert to prevent duplicates)
		_, err := collection.UpdateOne(
			context.TODO(),
			bson.M{"url": page.URL},
			bson.M{"$set": page},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Println("Error saving to MongoDB:", err)
		} else {
			fmt.Println("✅ Saved:", page.URL)
		}
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if link != "" && !skipPattern.MatchString(link) {
			c.Visit(link)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("🔗 Visiting:", r.URL.String())
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error on %s: %v", r.Request.URL, err)
	})

	startURL := "https://golang.org"
	fmt.Println("🚀 Starting crawl on:", startURL)
	if err := c.Visit(startURL); err != nil {
		log.Fatal("Visit error:", err)
	}

	c.Wait()
	fmt.Println("\n Crawl complete! All data stored in MongoDB.")

	exportToJSON(collection)
}

func exportToJSON(collection *mongo.Collection) {
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Println("Error fetching from MongoDB:", err)
		return
	}
	defer cursor.Close(context.TODO())

	var pages []PageData
	if err = cursor.All(context.TODO(), &pages); err != nil {
		log.Println("Cursor decode error:", err)
		return
	}

	file, err := os.Create("results.json")
	if err != nil {
		log.Println("Error creating JSON file:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(pages)

	fmt.Println("Exported backup to results.json")
}
