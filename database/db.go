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

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		"localhost",  // host
		"5433",       // port
		"postgres",   // user
		"postgres",   // password
		"darkweb_db", // dbname
	)

	if os.Getenv("DB_SOURCE") != "" {
		connStr = os.Getenv("DB_SOURCE")
	}

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to the database: ", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("Error pinging the database: ", err)
	}

	fmt.Println("Database connection established.")
	createTables()

}

func createTables() {
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
            matches TEXT,      -- YENİ EKLENDİ
            screenshot TEXT,   -- YENİ EKLENDİ
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );`

	_, err := DB.Exec(query)
	if err != nil {
		log.Fatal("Error creating tables: ", err)
	}

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

	fmt.Println("Tables created or already exist.")

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
}
