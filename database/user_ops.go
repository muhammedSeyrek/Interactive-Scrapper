package database

import (
	"database/sql"
	"interactive-scraper/models"
	"log"
)

func CreateUser(user *models.User) error {
	query := `INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id`
	err := DB.QueryRow(query, user.Username, user.PasswordHash, user.Role).Scan(&user.ID)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return err
	}
	return nil
}

func GetUserByUsername(username string) (*models.User, error) {
	query := `SELECT id, username, password_hash, role FROM users WHERE username = $1`

	row := DB.QueryRow(query, username)

	var user models.User
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
