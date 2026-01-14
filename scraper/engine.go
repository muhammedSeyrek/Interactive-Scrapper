package scraper

import (
	"context"
	"encoding/base64"
	"fmt"
	"interactive-scraper/models"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Regex definition for previous compiled to performance
var (
	emailRegex  = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	cryptoRegex = regexp.MustCompile(`(bc1|[13])[a-km-zA-HJ-NP-Z1-9]{25,34}`) // Basic BTC regex
	btcRegex    = regexp.MustCompile(`(bc1|[13])[a-km-zA-HJ-NP-Z1-9]{25,34}`) // Bitcoin
	ethRegex    = regexp.MustCompile(`0x[a-fA-F0-9]{40}`)                     // Ethereum
	xmrRegex    = regexp.MustCompile(`4[0-9AB][1-9A-HJ-NP-Za-km-z]{93}`)      // Monero (Dark web's king)
	onionRegex  = regexp.MustCompile(`[a-z2-7]{16,56}\.onion`)
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
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.ProxyServer(proxyAdd),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var title, content string
	var buf []byte

	fmt.Printf("Is starting scraping for URL: %s\n", targetURL)

	err := chromedp.Run(ctx, chromedp.Navigate(targetURL), chromedp.Title(&title), chromedp.OuterHTML("html", &content),
		chromedp.CaptureScreenshot(&buf))

	if err != nil {
		return nil, fmt.Errorf("failed to scrape URL: %v", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("timeout/context error: %w", err)
	}

	score, category, findings := AnalyzeContent(content, title)

	if title == "" {
		title = fmt.Sprintf("Tor Onion site: %s", targetURL)
	}

	// Convert the image to base64.
	encodedScreenshot := ""
	if len(buf) > 0 {
		encodedScreenshot = base64.StdEncoding.EncodeToString(buf)
	}

	cleanContent := strings.Join(strings.Fields(content), " ")
	parsedURL, err := url.Parse(targetURL)
	sourceName := parsedURL.Hostname()

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
		Matches:          findings,
		Screenshot:       encodedScreenshot,
	}

	return result, nil

}

func cleanContent(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}

func AnalyzeContent(text string, title string) (int, string, string) {
	textLower := strings.ToLower(text + " " + title)
	score := 1
	cat := "General Info"
	var foundList []string

	keywords := map[string]int{
		"hacked": 5, "leaked": 6, "database": 6, "dump": 5, "sql injection": 7,
		"drug": 7, "cocaine": 8, "weed": 4, "market": 5, "cartel": 8,
		"passport": 9, "id card": 9, "ssn": 10, "cc": 8, "cvv": 9, "fullz": 10,
		"weapon": 8, "gun": 8, "glock": 7, "hitman": 10,
		"ddos": 6, "botnet": 7, "exploit": 7, "0day": 9, "malware": 8, "ransomware": 9,
		"def con": 3, "conference": 2, "security": 2,
	}

	for word, points := range keywords {
		if strings.Contains(textLower, word) {
			score += points
			// Let's add risky keywords to the intelligence list as well, so we can why it's given a score.
			if points >= 5 {
				foundList = append(foundList, "Keyword: "+word)
			}
		}
	}

	// finding email
	emails := emailRegex.FindAllString(text, -1)
	if len(emails) > 0 {
		score += 3 // first 3 email recording, because memory inflatable.
		foundList = append(foundList, fmt.Sprintf("Emails: %v", uniqueStrings(emails)))
		cat = "Communication / Leaks"
	}

	// finding crypto wallet
	btc := btcRegex.FindAllString(text, -1)
	eth := ethRegex.FindAllString(text, -1)
	xmr := xmrRegex.FindAllString(text, -1)

	if len(btc) > 0 || len(eth) > 0 || len(xmr) > 0 {
		score += 4
		cat = "Financial"
		if len(btc) > 0 {
			foundList = append(foundList, fmt.Sprintf("BTC: %v", uniqueStrings(btc)))
		}
		if len(eth) > 0 {
			foundList = append(foundList, fmt.Sprintf("ETH: %v", uniqueStrings(eth)))
		}
		if len(xmr) > 0 {
			foundList = append(foundList, fmt.Sprintf("XMR: %v", uniqueStrings(xmr)))
		}
	}

	onions := onionRegex.FindAllString(text, -1)
	if len(onions) > 5 { // If much have links, it would be a "link list" site.
		foundList = append(foundList, fmt.Sprintf("Found %d Onion Links", len(onions)))
	}

	if score > 10 {
		score = 10
	}

	if score >= 8 {
		cat = "Critical Threat"
	} else if score >= 5 {
		cat = "Suspicious"
	}

	matchesStr := strings.Join(foundList, " | ")

	if len(matchesStr) > 500 {
		matchesStr = matchesStr[:497] + "..."
	}

	return score, cat, matchesStr

}

// Clean up duplicate data.
func uniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			// Take the first 3 so db doesn't get inflated.
			if len(list) < 3 {
				list = append(list, entry)
			}
		}
	}
	return list
}

func ExtractOnionLinks(htmlContent string, baseURL string) []string {
	// Basic href regex
	linkRegex := regexp.MustCompile(`href=["'](.*?)["']`)
	matches := linkRegex.FindAllStringSubmatch(htmlContent, -1)

	var links []string
	base, _ := url.Parse(baseURL)

	for _, match := range matches {
		link := match[1]

		if strings.HasPrefix(link, "/") {
			link = fmt.Sprintf("%s://%s%s", base.Scheme, base.Host, link)
		} else if !strings.HasPrefix(link, "http") {

			continue
		}

		if strings.Contains(link, base.Host) && strings.Contains(link, ".onion") {

			if link != baseURL {
				links = append(links, link)
			}
		}
	}
	return uniqueStrings(links)
}
