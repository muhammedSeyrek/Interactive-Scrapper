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
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`

	_, err := DB.Exec(query)
	if err != nil {
		log.Fatal("Error creating tables: ", err)
	}
	fmt.Println("Tables created or already exist.")
}
