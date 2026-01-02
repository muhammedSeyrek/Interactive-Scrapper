package main

import (
	"fmt"
	"html/template"
	"interactive-scraper/database"
	"interactive-scraper/models"
	"interactive-scraper/scraper"
	"log"
	"net/http"
	"strconv"
	"time"
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
	Contents []models.DarkWebContent
	Stats    DashboardStats
}

type DashboardStats struct {
	Total      int
	HighRisk   int
	MediumRisk int
	LowRisk    int
}

func main() {

	database.InitDB()

	go func() {
		contents, _ := database.GetAllContent()
		if len(contents) == 0 {
			fmt.Println("Database is empty, starting initial scrape...")
			data, err := scraper.ScrapeURL("https://ibm.com") // Replace with actual URL
			if err == nil {
				database.SaveDarkWebContent(data)
			}
		}
	}()

	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/view", authMiddleware(viewHandler))
	http.HandleFunc("/", authMiddleware(dashboardHandler))

	port := ":8080"
	fmt.Printf("Starting server on port: http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))

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
	// Fetch all content from the database
	contents, err := database.GetAllContent()
	if err != nil {
		http.Error(w, "Failed to load content: "+err.Error(), http.StatusInternalServerError)
	}

	stats := DashboardStats{
		Total: len(contents),
	}

	for _, c := range contents {
		if c.CriticalityScore >= 7 {
			stats.HighRisk++
		} else if c.CriticalityScore >= 4 {
			stats.MediumRisk++
		} else {
			stats.LowRisk++
		}
	}

	data := PageData{
		Contents: contents,
		Stats:    stats,
	}

	tmpl, err := template.ParseFiles("templates/index.html")

	if err != nil {
		http.Error(w, "Failed to parse template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, data)

}
