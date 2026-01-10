package scraper

import (
	"context"
	"fmt"
	"interactive-scraper/models"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func ScrapeURL(targetURL string) (*models.DarkWebContent, error) {

	proxyAdd := os.Getenv("TOR_PROXY")
	if proxyAdd == "" {
		proxyAdd = "socks5://127.0.0.1:9050"
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.ProxyServer(proxyAdd),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var title, content string

	fmt.Printf("Is starting scraping for URL: %s\n", targetURL)

	err := chromedp.Run(ctx, chromedp.Navigate(targetURL), chromedp.Title(&title), chromedp.Text("body", &content))

	if err != nil {
		return nil, fmt.Errorf("failed to scrape URL: %v", err)
	}

	score, category := AnalyzeContent(content)

	if title == "" {
		title = fmt.Sprintf("Tor Onion site: %s", targetURL)
	}

	cleanContent := strings.Join(strings.Fields(content), " ")

	parsedURL, err := url.Parse(targetURL)
	sourceName := "Unknown Source"
	if err == nil {
		sourceName = parsedURL.Hostname()
	}

	result := &models.DarkWebContent{
		SourceName:       sourceName,
		SourceURL:        targetURL,
		Content:          cleanContent,
		Title:            title,
		PublishedDate:    time.Now(),
		CriticalityScore: score,
		Category:         category,
	}

	return result, nil

}

func cleanContent(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}

func AnalyzeContent(text string) (int, string) {
	text = strings.ToLower(text)
	score := 1
	cat := "General Info"

	keywords := map[string]int{
		"hacked": 5, "leaked": 6, "database": 5,
		"drug": 7, "cocaine": 8, "weed": 4,
		"passport": 9, "id card": 9, "ssn": 10,
		"weapon": 8, "gun": 8,
		"ddos": 6, "botnet": 7,
	}

	for word, points := range keywords {
		if strings.Contains(text, word) {
			score += points

			// Cap the score at 8
			if points >= 8 {
				cat = "Critical Threat"
			} else if points >= 5 && cat == "General" {
				cat = "Suspicious"
			} else {
				cat = "Low Risk / General"
			}

		}
	}
	// Limit the maximum score to 10
	if score > 10 {
		score = 10
	}

	return score, cat

}
