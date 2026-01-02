package database

import (
	"interactive-scraper/models"
	"log"
)

func UpdateContent(id int, score int, category string) error {
	query := `UPDATE dark_web_contents SET criticality_score = $1, category = $2 WHERE id = $3`
	_, err := DB.Exec(query, score, category, id)
	return err
}

func GetContentByID(id int) (*models.DarkWebContent, error) {
	query := `
    SELECT id, source_name, source_url, content, title, published_date, criticality_score, category 
    FROM dark_web_contents 
    WHERE id = $1`

	var c models.DarkWebContent

	// Execute the query
	err := DB.QueryRow(query, id).Scan(&c.ID, &c.SourceName, &c.SourceURL, &c.Content, &c.Title,
		&c.PublishedDate, &c.CriticalityScore, &c.Category,
	)

	if err != nil {
		return nil, err
	}

	return &c, nil

}

func SaveDarkWebContent(data *models.DarkWebContent) error {
	query := `
	INSERT INTO dark_web_contents (source_name, source_url, content, title, published_date, criticality_score, category)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id`

	err := DB.QueryRow(query,
		data.SourceName,
		data.SourceURL,
		data.Content,
		data.Title,
		data.PublishedDate,
		data.CriticalityScore,
		data.Category,
	).Scan(&data.ID)

	if err != nil {
		log.Printf("Error saving DarkWebContent: %v", err)
		return err
	}

	log.Printf("DarkWebContent saved with ID: %d", data.ID)

	return nil

}

func GetAllContent() ([]models.DarkWebContent, error) {
	query := `
	SELECT id, source_name, source_url, content, title, published_date, criticality_score, category 
	FROM dark_web_contents 
	ORDER BY published_date DESC`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []models.DarkWebContent

	for rows.Next() {
		var c models.DarkWebContent
		if err := rows.Scan(&c.ID, &c.SourceName, &c.SourceURL, &c.Content, &c.Title,
			&c.PublishedDate, &c.CriticalityScore, &c.Category); err != nil {
			return nil, err
		}
		contents = append(contents, c)
	}

	return contents, nil
}
