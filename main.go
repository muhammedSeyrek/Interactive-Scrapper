package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"interactive-scraper/auth"
	"interactive-scraper/database"
	"interactive-scraper/models"
	"interactive-scraper/reports"
	"interactive-scraper/scraper"
	"interactive-scraper/services"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gopkg.in/yaml.v3"
)

type LoginData struct {
	Error string
}

// DashboardStats holds statistics for the dashboard
type PageData struct {
	Contents    []models.DarkWebContent
	Targets     []models.Target
	Stats       DashboardStats
	SearchQuery string
}

type DashboardStats struct {
	Total      int
	HighRisk   int
	MediumRisk int
	LowRisk    int
}

type Config struct {
	Targets []string `yaml:"targets"`
}

func main() {

	database.InitDB()
	loadTargetsFromYAML()

	go startAutomaticScanning()

	go func() {
		time.Sleep(20 * time.Second)
		contents, _ := database.GetAllContent()
		if len(contents) == 0 {
			fmt.Println("[INIT] Database is empty, starting initial scrape from targets...")
			targets, err := database.GetAllTargets()
			if err != nil || len(targets) == 0 {
				fmt.Println("[INIT] No targets found")
				return
			}

			// Scan first 3 targets
			maxTargets := 3
			if len(targets) < maxTargets {
				maxTargets = len(targets)
			}

			for i := 0; i < maxTargets; i++ {
				target := targets[i]
				fmt.Printf("[INIT] Scanning: %s\n", target.URL)

				data, err := scraper.ScrapeURL(target.URL)
				if err == nil {
					database.SaveDarkWebContent(data)
					if data.ID != 0 {
						database.SaveEntities(data.ID, data.Entities)
					}
					fmt.Printf("[INIT] Saved: %s\n", target.URL)
				} else {
					fmt.Printf("[INIT] Failed: %s - %v\n", target.URL, err)
				}

				time.Sleep(5 * time.Second) // Rate limiting
			}
			fmt.Println("[INIT] Initial scrape completed")
		}
	}()

	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/view", auth.Middleware(viewHandler))
	http.HandleFunc("/add-target", auth.Middleware(addTargetHandler))
	http.HandleFunc("/delete-target", auth.Middleware(deleteTargetHandler))
	http.HandleFunc("/targets", auth.Middleware(targetsHandler))
	http.HandleFunc("/scan-now", auth.Middleware(scanNowHandler))
	http.HandleFunc("/deep-scan", auth.Middleware(deepScanHandler))
	http.HandleFunc("/export/json", auth.Middleware(exportJSONHandler))
	http.HandleFunc("/export/pdf", auth.Middleware(exportPDFHandler))
	http.HandleFunc("/graph", auth.Middleware(graphPageHandler))
	http.HandleFunc("/api/graph", auth.Middleware(graphDataHandler))
	// API endpoints - Protected with auth
	http.HandleFunc("/api/targets", apiMiddleware(apiTargetsHandler))
	http.HandleFunc("/api/scan", apiMiddleware(apiScanHandler))
	http.HandleFunc("/api/content", apiMiddleware(apiContentHandler))
	http.HandleFunc("/api/changes", apiMiddleware(apiChangesHandler))
	http.HandleFunc("/", auth.Middleware(dashboardHandler))

	port := "0.0.0.0:8080"
	fmt.Printf("Starting server on port: %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))

}

func loadTargetsFromYAML() {

	file, err := ioutil.ReadFile("targets.yaml")
	if err != nil {
		fmt.Println("Targets.yaml is founded")
		return
	}

	var config Config
	if err := yaml.Unmarshal(file, &config); err != nil {
		fmt.Println("YAML Format error:", err)
		return
	}

	count := 0
	for _, url := range config.Targets {
		if url != "" {
			err := database.AddTarget(url, "yaml")
			if err == nil {
				count++
			}
		}
	}
	fmt.Printf("Target %d was loaded/updated from the YAML file.\n", count)
}

func scanNowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		targetID, _ := strconv.Atoi(r.FormValue("id"))
		target, err := database.GetTargetByID(targetID)
		if err != nil {
			fmt.Println("Target isn't found:", err)
			http.Redirect(w, r, "/targets", http.StatusSeeOther)
			return
		}

		fmt.Printf("Debug is started: %s", target.URL)
		err = database.UpdateTargetStatus(target.ID, "Scanning...")
		if err != nil {
			log.Printf("DB state error to update: %v", err)
		}

		fmt.Println("Manual scan is started:", target.URL)

		database.UpdateTargetStatus(target.ID, "Scanning...")

		go func(t models.Target) {
			data, err := scraper.ScrapeURL(t.URL)
			if err != nil {
				database.UpdateTargetStatus(t.ID, "Failed")
				fmt.Printf("[%s] scan failed: %v\n", t.URL, err)
			} else {
				saveErr := database.SaveDarkWebContent(data)
				if saveErr == nil && data.ID != 0 {
					database.SaveEntities(data.ID, data.Entities)
					fmt.Printf(" -> %d entities saved for ID %d\n", len(data.Entities), data.ID)
				}
				if saveErr != nil {
					fmt.Printf("!!! DB SAVE ERROR for %s: %v\n", t.URL, saveErr)
					database.UpdateTargetStatus(t.ID, "DB Error")
				} else {
					database.UpdateTargetStatus(t.ID, "Online")
					fmt.Printf("[%s] scan success and saved.\n", t.URL)
				}
			}
		}(target)

	}
	http.Redirect(w, r, "/targets", http.StatusSeeOther)
}

func addTargetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		url := r.FormValue("url")
		if len(url) > 0 {
			database.AddTarget(url, "manual")
			fmt.Println("New target added:", url)
		}
	}
	http.Redirect(w, r, "/targets", http.StatusSeeOther)
}

func deleteTargetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		idStr := r.FormValue("id")
		id, _ := strconv.Atoi(idStr)

		database.DeleteTarget(id)
		fmt.Println("Target deleted ID:", id)
	}
	http.Redirect(w, r, "/targets", http.StatusSeeOther)
}

func viewHandler(w http.ResponseWriter, r *http.Request) {

	// Get content ID from query parameters
	idStr := r.URL.Query().Get("id")

	if idStr == "" {
		http.Error(w, "Missing content ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid content ID", http.StatusBadRequest)
		return
	}

	if r.Method == "POST" {
		newScore, _ := strconv.Atoi(r.FormValue("score"))
		newCategory := r.FormValue("category")

		err := database.UpdateContent(id, newScore, newCategory)
		if err != nil {
			http.Error(w, "Failed to update content: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return

	}

	content, err := database.GetContentByID(id)
	if err != nil {
		http.Error(w, "Content not found: "+err.Error(), http.StatusNotFound)
		return
	}

	tmpl, err := template.ParseFiles("templates/detail.html")
	if err != nil {
		http.Error(w, "Failed to parse template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, content)

}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	cookie, err := r.Cookie("auth_token")
	if err == nil && cookie.Value != "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		tokenCookie, err := auth.Login(username, password)
		if err != nil {
			tmpl, _ := template.ParseFiles("templates/login.html")
			tmpl.Execute(w, LoginData{Error: "Incorrect username or wrong password!"})
			return
		}
		http.SetCookie(w, tokenCookie)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl, _ := template.ParseFiles("templates/login.html")
	tmpl.Execute(w, nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "auth_token",
		Value:   "",
		Expires: time.Now().Add(-1 * time.Hour),
		MaxAge:  -1,
		Path:    "/",
	})

	fmt.Println("User logged out.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {

	targets, _ := database.GetAllTargets()

	query := r.URL.Query().Get("q")

	var contents []models.DarkWebContent
	var err error

	if query != "" {
		contents, err = database.SearchContent(query)
	} else {
		contents, err = database.GetAllContent()
	}

	if err != nil {
		log.Printf("Error getting content: %v", err)
		http.Error(w, "Failed to load content", http.StatusInternalServerError)
		return
	}

	stats := DashboardStats{
		Total: len(contents),
	}

	for _, c := range contents {
		if c.CriticalityScore >= 8 {
			stats.HighRisk++
		} else if c.CriticalityScore >= 5 {
			stats.MediumRisk++
		} else {
			stats.LowRisk++
		}
	}

	data := PageData{
		Contents:    contents,
		Targets:     targets,
		Stats:       stats,
		SearchQuery: query,
	}

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Failed to parse template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, data)

}

func targetsHandler(w http.ResponseWriter, r *http.Request) {

	targets, err := database.GetAllTargets()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Veriyi sayfaya gönder
	data := struct {
		Targets []models.Target
	}{
		Targets: targets,
	}

	tmpl, err := template.ParseFiles("templates/targets.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

func deepScanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		idStr := r.FormValue("id")
		targetID, _ := strconv.Atoi(idStr)
		target, err := database.GetTargetByID(targetID)

		if err != nil {
			http.Redirect(w, r, "/targets", http.StatusSeeOther)
			return
		}

		fmt.Printf("[DEEP SCAN] Started for: %s\n", target.URL)
		database.UpdateTargetStatus(target.ID, "Deep Scanning...")

		go func(t models.Target) {

			mainResult, err := scraper.ScrapeURL(t.URL)
			if err != nil && mainResult.ID != 0 {
				database.UpdateTargetStatus(t.ID, "Failed")
				return
			}
			saveErr := database.SaveDarkWebContent(mainResult)

			if saveErr == nil && t.ID != 0 {
				database.SaveEntities(mainResult.ID, mainResult.Entities)
				fmt.Printf(" -> %d entities saved for ID %d\n", len(mainResult.Entities), mainResult.ID)
			}

			links := scraper.ExtractOnionLinks(mainResult.Content, t.URL)

			for _, foundLink := range links {
				dbErr := database.AddLinkRelationship(t.URL, foundLink)
				if dbErr != nil {
					fmt.Printf("Relationship save error: %v\n", dbErr)
				}
			}

			maxLinks := 5
			if len(links) < maxLinks {
				maxLinks = len(links)
			}
			subLinks := links[:maxLinks]

			fmt.Printf("[DEEP SCAN] Found %d links. Scanning top %d...\n", len(links), len(subLinks))

			for _, link := range subLinks {
				fmt.Printf("   -> Spawning worker for: %s\n", link)

				subResult, subErr := scraper.ScrapeURL(link)
				if subErr == nil {
					subResult.Title = "[SUB] " + subResult.Title
					subResult.Category = subResult.Category + " (DeepScan)"

					database.SaveDarkWebContent(subResult)
				}
			}

			database.UpdateTargetStatus(t.ID, "Online (Deep Scanned)")
			fmt.Printf("[DEEP SCAN] Finished for %s\n", t.URL)

		}(target)
	}
	http.Redirect(w, r, "/targets", http.StatusSeeOther)
}

func exportJSONHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	content, err := database.GetContentByID(id)
	if err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	jsonData, err := reports.GenerateJSON(content)
	if err != nil {
		http.Error(w, "JSON generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=report_%d.json", id))
	w.Write(jsonData)
}

func exportPDFHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	content, err := database.GetContentByID(id)
	if err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	pdfBuf, err := reports.GeneratePDF(content)
	if err != nil {
		log.Printf("PDF Error: %v", err)
		http.Error(w, "PDF generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=threat_report_%d.pdf", id))
	w.Write(pdfBuf.Bytes())
}

func graphDataHandler(w http.ResponseWriter, r *http.Request) {
	nodes, edges, err := database.GetGraphData()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "` + err.Error() + `"}`))
		return
	}

	if nodes == nil {
		nodes = []models.GraphNode{}
	}
	if edges == nil {
		edges = []models.GraphEdge{}
	}

	response := struct {
		Nodes []models.GraphNode `json:"nodes"`
		Edges []models.GraphEdge `json:"edges"`
	}{
		Nodes: nodes,
		Edges: edges,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func graphPageHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/graph.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Background scanner runs automatically every 6 hours
func startAutomaticScanning() {
	fmt.Println("[BACKGROUND] Scanner initialized. Will scan every 6 hours...")
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	// First scan after 2 minutes startup
	time.Sleep(2 * time.Minute)
	performBackgroundScan()

	for range ticker.C {
		fmt.Println("[BACKGROUND] 6-hour interval reached. Starting scan...")
		performBackgroundScan()
	}
}

// performBackgroundScan scans all targets and detects changes
func performBackgroundScan() {
	targets, err := database.GetAllTargets()
	if err != nil {
		fmt.Printf("[BACKGROUND] Error fetching targets: %v\n", err)
		return
	}

	if len(targets) == 0 {
		fmt.Println("[BACKGROUND] No targets to scan")
		return
	}

	fmt.Printf("[BACKGROUND] Starting scan of %d targets...\n", len(targets))

	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func(target models.Target) {
			defer wg.Done()
			scanTargetWithDiff(target)
		}(t)
	}

	wg.Wait()
	fmt.Println("[BACKGROUND] Scan cycle completed")
}

// scanTargetWithDiff scans a target, checks URL reputation, detects changes, and enriches data with VirusTotal

func scanTargetWithDiff(target models.Target) {
	// ---------------------------------------------------------
	// CONFIGURATION (Sensitive data should ideally be in env vars)
	// ---------------------------------------------------------
	apiKey := os.Getenv("VIRUSTOTAL_API_KEY")
	slackWebhook := os.Getenv("SLACK_WEBHOOK_URL")

	if apiKey == "" || slackWebhook == "" {
		fmt.Println("[WARNING] API Key or Webhook URL not set in environment variables!")
	}

	// STEP 1: Check Target URL Reputation (Domain Level)
	// Before scraping, ask VirusTotal if the site itself is malicious.
	fmt.Printf("[VT] Checking reputation for: %s\n", target.URL)
	vtUrlReport, badUrlScore, _ := services.CheckURLReputation(target.URL, apiKey)

	if badUrlScore > 0 {
		fmt.Printf("🚨 DANGER! The target URL itself is malicious! (%d engines flagged it)\n", badUrlScore)

		// Immediate Alert for Malicious Domain
		details := fmt.Sprintf("🔴 DOMAIN BLOCKED BY VIRUSTOTAL!\nReport: %s", vtUrlReport)
		reports.SendSlackAlert(slackWebhook, target.URL+" (MALICIOUS DOMAIN DETECTED)", 10, 0, details)
	} else {
		fmt.Printf("[VT] Target seems clean: %s\n", vtUrlReport)
	}

	database.UpdateTargetStatus(target.ID, "Auto-Scanning...")

	// STEP 2: Scrape the Target Content
	data, err := scraper.ScrapeURL(target.URL)
	if err != nil {
		fmt.Printf("[BACKGROUND] Scan failed for %s: %v\n", target.URL, err)
		database.UpdateTargetStatus(target.ID, "Failed")
		return
	}

	// If the URL itself was malicious, force high risk score
	if badUrlScore > 0 {
		data.CriticalityScore = 10
		data.Category = "Known Malicious Site"
	}

	// STEP 3: Change Detection (Diffing)
	prevScan, _ := database.GetPreviousScan(target.URL, time.Now())
	var newEntities []models.EntityChange

	if prevScan != nil {
		newEntities = database.DetectNewEntities(data.Entities, prevScan.Entities)

		// Alert Condition: High Risk Score OR New Entities Found
		if len(newEntities) > 0 || data.CriticalityScore >= 5 {

			fmt.Printf("[BACKGROUND] Alert Triggered for %s (Score: %d, New: %d)\n", target.URL, data.CriticalityScore, len(newEntities))

			// --- BUILD INTELLIGENCE REPORT ---
			reportDetails := fmt.Sprintf("📋 **Findings:** %s\n", data.Matches)
			reportDetails += fmt.Sprintf("🌐 **Domain Status:** %s\n", vtUrlReport)

			// STEP 4: Entity Enrichment (Check IPs found inside content)
			if len(newEntities) > 0 {
				for _, e := range newEntities {
					if e.Type == "IP_ADDRESS" {
						vtReport, badScore, _ := services.CheckIPReputation(e.Value, apiKey)

						if badScore > 0 {
							reportDetails += fmt.Sprintf("🚫 **Malicious IP:** %s (%s)\n", e.Value, vtReport)
							fmt.Printf("🚨 MALICIOUS IP FOUND: %s\n", e.Value)
						} else {
							reportDetails += fmt.Sprintf("✅ Clean IP: %s\n", e.Value)
							fmt.Printf("[VT] Clean IP found: %s\n", e.Value)
						}
					}
					// Log finding to console
					fmt.Printf("  -> New Entity: [%s] %s\n", e.Type, e.Value)
				}
			}
			// -----------------------------

			// Send Detailed Slack Notification
			err := reports.SendSlackAlert(slackWebhook, target.URL, data.CriticalityScore, len(newEntities), reportDetails)
			if err != nil {
				fmt.Printf("Slack Notification Error: %v\n", err)
			} else {
				fmt.Println("-> Slack alert sent SUCCESSFULLY.")
			}
		}
	}

	// STEP 5: Save Results to Database
	saveErr := database.SaveDarkWebContent(data)
	if saveErr == nil && data.ID != 0 {
		database.SaveEntities(data.ID, data.Entities)
		database.UpdateTargetStatus(target.ID, "Online")
	} else {
		database.UpdateTargetStatus(target.ID, "DB Error")
	}

	// Rate limiting (Politeness delay)
	time.Sleep(10 * time.Second)
}

// apiTargetsHandler returns all targets as JSON
func apiTargetsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	targets, err := database.GetAllTargets()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(targets)
}

// apiContentHandler returns content with optional filtering
func apiContentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	var contents []models.DarkWebContent
	var err error

	if query != "" {
		contents, err = database.SearchContent(query)
	} else {
		contents, err = database.GetAllContent()
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Optional: filter by risk level
	riskLevel := r.URL.Query().Get("risk")
	if riskLevel != "" {
		var filtered []models.DarkWebContent
		for _, c := range contents {
			level := database.GetRiskLevel(c.CriticalityScore)
			if level == riskLevel {
				filtered = append(filtered, c)
			}
		}
		contents = filtered
	}

	json.NewEncoder(w).Encode(contents)
}

// apiScanHandler manually triggers a scan for a specific target
func apiScanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "POST required"})
		return
	}

	targetID := r.URL.Query().Get("id")
	id, err := strconv.Atoi(targetID)
	if err != nil || id == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Valid target ID required"})
		return
	}

	target, err := database.GetTargetByID(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Target not found"})
		return
	}

	// Trigger scan in background
	go scanTargetWithDiff(target)

	json.NewEncoder(w).Encode(map[string]string{"status": "Scan started", "target": target.URL})
}

// apiChangesHandler returns recently detected changes across all scans
func apiChangesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hoursStr := r.URL.Query().Get("hours")
	hours := 24 // default: last 24 hours
	if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
		hours = h
	}

	// Get all recent content from the specified time period
	query := fmt.Sprintf(`
    SELECT id, source_url, title, criticality_score, category, published_date
    FROM dark_web_contents
    WHERE published_date > NOW() - INTERVAL '%d hours'
    ORDER BY published_date DESC
    LIMIT 100
    `, hours)

	rows, err := database.DB.Query(query)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var changes []map[string]interface{}
	for rows.Next() {
		var id int
		var url, title, category string
		var score int
		var pubDate time.Time

		if err := rows.Scan(&id, &url, &title, &score, &category, &pubDate); err != nil {
			continue
		}

		changes = append(changes, map[string]interface{}{
			"id":       id,
			"url":      url,
			"title":    title,
			"score":    score,
			"level":    database.GetRiskLevel(score),
			"category": category,
			"scanned":  pubDate,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"period_hours": hours,
		"changes":      changes,
	})
}

// apiMiddleware checks auth for API endpoints and returns JSON errors
func apiMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		cookie, err := r.Cookie("auth_token")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		tokenStr := cookie.Value
		claims := &auth.Claims{}

		tkn, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return auth.GetJWTKey(), nil
		})

		if err != nil || !tkn.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
			return
		}

		next(w, r)
	}
}
