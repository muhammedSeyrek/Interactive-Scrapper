package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"interactive-scraper/database"
	"interactive-scraper/models"
	"interactive-scraper/reports"
	"interactive-scraper/scraper"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	AdminUser = "admin"
	AdminPass = "admin"
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

	//go startBackgroundScrapping()

	go func() {
		time.Sleep(20 * time.Second)
		contents, _ := database.GetAllContent()
		if len(contents) == 0 {
			fmt.Println("Database is empty, starting initial scrape...")
			url := "http://check.torproject.org" // Replace with actual .onion URL
			data, err := scraper.ScrapeURL(url)
			if err == nil {
				database.SaveDarkWebContent(data)
			} else {
				fmt.Println("Initial scrape failed: ", err)
			}
		}
	}()

	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/view", authMiddleware(viewHandler))
	http.HandleFunc("/add-target", authMiddleware(addTargetHandler))
	http.HandleFunc("/delete-target", authMiddleware(deleteTargetHandler))
	http.HandleFunc("/targets", authMiddleware(targetsHandler))
	http.HandleFunc("/scan-now", authMiddleware(scanNowHandler))
	http.HandleFunc("/deep-scan", authMiddleware(deepScanHandler))
	http.HandleFunc("/export/json", authMiddleware(exportJSONHandler))
	http.HandleFunc("/export/pdf", authMiddleware(exportPDFHandler))
	http.HandleFunc("/graph", authMiddleware(graphPageHandler))
	http.HandleFunc("/api/graph", authMiddleware(graphDataHandler))

	http.HandleFunc("/", authMiddleware(dashboardHandler))

	port := ":8080"
	fmt.Printf("Starting server on port: http://localhost%s\n", port)
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

func startBackgroundScrapping() {
	fmt.Println("Tor service is started.... wait for 30 second...")
	time.Sleep(10 * time.Second)

	for {
		targets, err := database.GetAllTargets()
		if err != nil {
			fmt.Println("Targets were not achieved:", err)
			time.Sleep(1 * time.Minute)
			continue
		}
		if len(targets) == 0 {
			fmt.Println("No targets to scan. Wait for 1 minute...")
			continue
		}

		fmt.Println("%d number of targets are being scanned...\n", len(targets))

		for _, t := range targets {
			fmt.Println("Scanning: %s\n", t.URL)

			database.UpdateTargetStatus(t.ID, "Scanning...")

			data, err := scraper.ScrapeURL(t.URL)
			if err == nil {
				database.SaveDarkWebContent(data)
				database.UpdateTargetStatus(t.ID, "Online")
				fmt.Println("Recorded.")
			} else {
				fmt.Println("Scan failed: %v\n", err)
				database.UpdateTargetStatus(t.ID, "Failed")
			}

			time.Sleep(15 * time.Second)
		}

		fmt.Println("Entire list was scan. 5 minute break...")
		time.Sleep(5 * time.Minute)
	}
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

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != "logged_in_secure" {
			fmt.Println("Unauthorized access attempt.")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value == "logged_in_secure" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl, _ := template.ParseFiles("templates/login.html")

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")
		if username == AdminUser && password == AdminPass {
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    "logged_in_secure",
				Expires:  time.Now().Add(24 * time.Hour),
				HttpOnly: true,
				Path:     "/",
			})
			fmt.Println("User logged in: ", username)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		} else {
			tmpl.Execute(w, LoginData{Error: "Invalid credentials"})
			return
		}
	}
	tmpl.Execute(w, nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
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
			if err != nil {
				database.UpdateTargetStatus(t.ID, "Failed")
				return
			}
			database.SaveDarkWebContent(mainResult)

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
