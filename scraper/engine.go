package scraper

import (
	"context"
	"fmt"
	"interactive-scraper/models"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func ScrapeURL(targetURL string) (*models.DarkWebContent, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var title, content string

	fmt.Printf("Is starting scraping for URL: %s\n", targetURL)

	err := chromedp.Run(ctx, chromedp.Navigate(targetURL), chromedp.Title(&title), chromedp.Text("body", &content))

	if err != nil {
		return nil, fmt.Errorf("failed to scrape URL: %v", err)
	}

	if title == "" {
		title = fmt.Sprintf("Reported Content from %s", targetURL)
	}

	cleanContent := strings.Join(strings.Fields(content), " ")

	result := &models.DarkWebContent{
		SourceName:       "Web Scraper",
		SourceURL:        targetURL,
		Content:          cleanContent,
		Title:            title,
		PublishedDate:    time.Now(),
		CriticalityScore: 5,
		Category:         "General",
	}

	return result, nil

}
