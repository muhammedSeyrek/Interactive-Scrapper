package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {

	var err error

	// 1. Get configuration from Environment Variables (Secure Method)
	// These values come from your .env file via docker-compose
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Set default values for local testing if env vars are missing
	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432" // Standard Postgres port
	}
	if dbUser == "" {
		dbUser = "postgres"
	}
	if dbName == "" {
		dbName = "darkweb_db"
	}

	// Construct the connection string safely
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	// Override if a full connection string is provided (Optional)
	if os.Getenv("DB_SOURCE") != "" {
		connStr = os.Getenv("DB_SOURCE")
	}

	// Open connection
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to the database: ", err)
	}

	// Verify connection with Ping
	if err = DB.Ping(); err != nil {
		log.Fatal("Error pinging the database: ", err)
	}

	fmt.Println("Database connection established successfully.")

	// Create tables if they don't exist
	createTables()
	CreateDefaultAdmin()

}

func createTables() {
	// Table: Scraped Content
	query := `
		CREATE TABLE IF NOT EXISTS dark_web_contents (
			id SERIAL PRIMARY KEY,
			source_name VARCHAR(255) NOT NULL,
			source_url TEXT NOT NULL,
			content TEXT NOT NULL,
			title VARCHAR(255),
			published_date TIMESTAMP,
			criticality_score INT DEFAULT 0,
			category VARCHAR(100),
			matches TEXT,      -- Newly Added: Stores MITRE/Keyword findings
			screenshot TEXT,   -- Newly Added: Base64 screenshot string
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`

	_, err := DB.Exec(query)
	if err != nil {
		log.Fatal("Error creating dark_web_contents table: ", err)
	}

	// Table: Target URLs
	queryTargets := `
		CREATE TABLE IF NOT EXISTS targets (
			id SERIAL PRIMARY KEY,
			url TEXT NOT NULL UNIQUE, 
			source VARCHAR(50) DEFAULT 'manual',
			last_status VARCHAR(50) DEFAULT 'Pending',
			last_scanned_at TIMESTAMP,                  
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`

	_, err = DB.Exec(queryTargets)
	if err != nil {
		log.Fatal("Error creating targets table: ", err)
	}

	// Table: Link Relationships (For Graph View)
	queryRelations := `
		CREATE TABLE IF NOT EXISTS link_relationships (
			id SERIAL PRIMARY KEY,
			source_url TEXT NOT NULL, 
			target_url TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(source_url, target_url)
		);`

	_, err = DB.Exec(queryRelations)
	if err != nil {
		log.Fatal("Error creating link_relationships table: ", err)
	}

	// Table: Users (For Auth)
	queryUsers := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role VARCHAR(20) DEFAULT 'user',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`

	_, err = DB.Exec(queryUsers)
	if err != nil {
		log.Fatal("Error creating users table: ", err)
	}

	fmt.Println("Tables checked/created successfully.")
}

func CreateDefaultAdmin() {
	var count int

	row := DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'")
	err := row.Scan(&count)
	if err != nil {
		log.Printf("Error checking admin user: %v", err)
		return
	}

	if count == 0 {
		fmt.Println("[INIT] Admin user not found. Creating default admin (admin / admin123)...")

		passwordHash := "$2a$10$EixZaYVK1fsbw1ZfbX3OXePaWxwKc.6I1.5ce6ygi.tI.yr9168bm"

		query := `INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3)`
		_, err := DB.Exec(query, "admin", passwordHash, "admin")
		if err != nil {
			log.Printf("Failed to create default admin: %v", err)
		} else {
			fmt.Println("[INIT] Default admin created successfully.")
		}
	} else {
		fmt.Println("[INIT] Admin user already exists.")
	}
}
