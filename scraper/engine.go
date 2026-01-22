package scraper

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"interactive-scraper/models"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type JsonRule struct {
	Keyword string `json:"keyword"`
	Score   int    `json:"score"`
	MitreID string `json:"mitre_id"`
	Tactic  string `json:"tactic"`
}

type ThreatRule struct {
	Score   int
	MitreID string
	Tactic  string
}

var loadedRules map[string]ThreatRule

// Regex definition for previous compiled to performance
var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	// Bitcoin: Both old one (1...), and Segwit (3...), and Bech32 (bc1...) formats
	btcRegex = regexp.MustCompile(`\b(bc1|[13])[a-km-zA-HJ-NP-Z1-9]{25,34}\b`)
	// Monero: King of dark web (It's start with 4... or 8... and both of them too long.)
	xmrRegex = regexp.MustCompile(`\b(4|8)[0-9AB][1-9A-HJ-NP-Za-km-z]{93}\b`)
	// Ethereum: It's starts with 0x
	ethRegex = regexp.MustCompile(`\b0x[a-fA-F0-9]{40}\b`)
	// Google Analytics/Tag Manager: critics for Deanonymization!
	gaRegex = regexp.MustCompile(`\b(UA-\d+-\d+|G-[A-Z0-9]+|GTM-[A-Z0-9]+)\b`)
	// PGP Key Block
	pgpRegex = regexp.MustCompile(`-----BEGIN PGP PUBLIC KEY BLOCK-----`)
	// IP Regex
	ipRegex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

func init() {
	loadRulesFromFile("mitre_rules.json")
}

func loadRulesFromFile(filename string) {
	loadedRules = make(map[string]ThreatRule)

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("[WARNING] MITRE rules file not found (%s). Using empty rules.\n", filename)
		return
	}

	var jsonRules []JsonRule
	if err := json.Unmarshal(file, &jsonRules); err != nil {
		fmt.Printf("[ERROR] Failed to parse MITRE rules: %v\n", err)
		return
	}

	for _, r := range jsonRules {
		loadedRules[r.Keyword] = ThreatRule{
			Score:   r.Score,
			MitreID: r.MitreID,
			Tactic:  r.Tactic,
		}
	}
	fmt.Printf("[INIT] Loaded %d MITRE rules from %s\n", len(loadedRules), filename)
}

// getThreatRules artık hafızadaki yüklü kuralları döndürür
func getThreatRules() map[string]ThreatRule {
	if len(loadedRules) == 0 {
		// Dosya yoksa veya boşsa yedek (fallback) birkaç kural
		return map[string]ThreatRule{
			"hacked": {6, "GENERIC", "Indicator"},
		}
	}
	return loadedRules
}

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
		chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36`),
	)

	if strings.Contains(targetURL, ".onion") {
		proxyAdd := os.Getenv("TOR_PROXY")
		if proxyAdd == "" {
			proxyAdd = "socks5://127.0.0.1:9050"
		}
		opts = append(opts, chromedp.ProxyServer(proxyAdd))
		fmt.Printf("[NETWORK] Tor mode is active: %s\n", targetURL)
	} else {
		fmt.Printf("[NETWORK] Clean Surface Web Mod (fast than tor): %s\n", targetURL)
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	timeout := 30 * time.Second
	if strings.Contains(targetURL, ".onion") {
		timeout = 90 * time.Second
	}

	ctx, cancel = context.WithTimeout(ctx, timeout)
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

	score, category, findings, entities := AnalyzeContent(content, title)

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
		Entities:         entities,
	}

	return result, nil

}

func cleanContent(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}

func AnalyzeContent(text string, title string) (int, string, string, []models.ExtractedEntity) {
	textLower := strings.ToLower(text + " " + title)
	score := 1
	cat := "General Info"
	var foundList []string

	var entities []models.ExtractedEntity

	keywords := map[string]int{
		// Data breaches & hacking
		"hacked": 8, "breached": 8, "leaked": 9, "data dump": 9, "stolen data": 9,
		"sql injection": 10, "vulnerability": 6, "zero day": 9, "exploit kit": 9,
		"credential stuffing": 9, "phishing": 7,

		// Illegal drugs & substances
		"cocaine": 10, "heroin": 10, "methamphetamine": 10, "fentanyl": 10,
		"ketamine": 9, "mdma": 8, "lsd": 8, "darknet drug": 10,

		// Dark markets & illegal commerce
		"dark market": 10, "black market": 10, "darknet market": 10,
		"for sale": 3, "marketplace": 2, // Low score - too generic

		// Stolen identities & financial fraud
		"passport": 10, "id card": 9, "ssn": 10, "social security": 10,
		"cc number": 10, "credit card": 8, "cvv": 9, "fullz": 10,
		"bank account": 9, "routing number": 9, "identity theft": 10,

		// Weapons & violence
		"weapon": 9, "gun": 7, "rifle": 8, "pistol": 8, "explosives": 10,
		"bomb": 10, "hitman": 10, "assassination": 10, "cartel": 10,

		// Cyber attacks & malware
		"ddos": 8, "botnet": 9, "malware": 9, "ransomware": 10, "spyware": 9,
		"trojan": 8, "worm": 7, "virus": 6, "keylogger": 9,

		// Illegal content
		"illegal content": 10, "forbidden": 7, "restricted": 5,

		// Low scoring (benign)
		"conference": 1, "security": 1, "forum": 2, "database": 3,
	}

	// \b means word limit. So if you're looking for "cc", you won't get the content of "success".
	// It only takes the word "cc" written separately.
	for word, points := range keywords {
		regex := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)

		if regex.MatchString(textLower) {
			score += points
			if points >= 5 {
				foundList = append(foundList, "Keyword: "+word)
			}
		}
	}

	// Entity extraction helper
	extract := func(regex *regexp.Regexp, typeName string, points int) {
		matches := regex.FindAllString(text, -1)

		// Do unique
		seen := make(map[string]bool)
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				entities = append(entities, models.ExtractedEntity{Type: typeName, Value: m})
				if len(seen) <= 3 {
					foundList = append(foundList, fmt.Sprintf("%s: %s", typeName, m))
				}
			}
		}
		if len(matches) > 0 {
			score += points
			if typeName == "BTC_WALLET" || typeName == "XMR_WALLET" {
				cat = "Financial / Market"
			}
		}

	}

	extract(emailRegex, "EMAIL", 3)
	extract(btcRegex, "BTC_WALLET", 5)
	extract(xmrRegex, "XMR_WALLET", 6)
	extract(ethRegex, "ETH_WALLET", 4)
	extract(gaRegex, "TRACKING_ID", 8)

	if pgpRegex.MatchString(text) {
		score += 2
		foundList = append(foundList, "PGP Key Detected")
		entities = append(entities, models.ExtractedEntity{Type: "PGP_KEY", Value: "PGP Block Present"})
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

	return score, cat, matchesStr, entities

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
	var links []string

	linkRegex := regexp.MustCompile(`href\s*=\s*["']([^"']+)["']`)
	matches := linkRegex.FindAllStringSubmatch(htmlContent, -1)

	base, err := url.Parse(baseURL)
	if err != nil {
		return links
	}

	seen := make(map[string]bool)

	for _, match := range matches {
		rawLink := strings.TrimSpace(match[1])

		if rawLink == "" || strings.HasPrefix(rawLink, "#") || strings.HasPrefix(rawLink, "javascript:") || strings.HasPrefix(rawLink, "mailto:") {
			continue
		}

		var fullURL string

		if strings.HasPrefix(rawLink, "http") {
			fullURL = rawLink
		} else if strings.HasPrefix(rawLink, "//") {
			fullURL = base.Scheme + ":" + rawLink
		} else {
			// Path birleştirme
			rel, err := url.Parse(rawLink)
			if err == nil {
				fullURL = base.ResolveReference(rel).String()
			} else {
				continue
			}
		}

		if strings.Contains(fullURL, ".onion") {
			if strings.Contains(fullURL, base.Host) {
				if fullURL != baseURL && !seen[fullURL] {
					links = append(links, fullURL)
					seen[fullURL] = true
				}
			}
		}
	}
	return links
}
