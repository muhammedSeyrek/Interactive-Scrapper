package database

import (
	"interactive-scraper/models"
	"log"
)

func UpdateTargetStatus(id int, status string) error {
	query := `UPDATE targets SET last_status = $1, last_scanned_at = NOW() WHERE id = $2`
	_, err := DB.Exec(query, status, id)
	return err
}

func UpdateContent(id int, score int, category string) error {
	query := `UPDATE dark_web_contents SET criticality_score = $1, category = $2 WHERE id = $3`
	_, err := DB.Exec(query, score, category, id)
	return err
}

func GetContentByID(id int) (*models.DarkWebContent, error) {
	query := `
    SELECT id, source_name, source_url, content, title, published_date, criticality_score, category, matches, COALESCE(screenshot, '')
    FROM dark_web_contents 
    WHERE id = $1`

	var c models.DarkWebContent

	// Execute the query
	err := DB.QueryRow(query, id).Scan(&c.ID, &c.SourceName, &c.SourceURL, &c.Content, &c.Title,
		&c.PublishedDate, &c.CriticalityScore, &c.Category, &c.Matches, &c.Screenshot,
	)

	if err != nil {
		return nil, err
	}

	return &c, nil

}

func SaveDarkWebContent(data *models.DarkWebContent) error {
	query := `
    INSERT INTO dark_web_contents (source_name, source_url, content, title, published_date, criticality_score, category, matches, screenshot)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id`

	err := DB.QueryRow(query,
		data.SourceName,
		data.SourceURL,
		data.Content,
		data.Title,
		data.PublishedDate,
		data.CriticalityScore,
		data.Category,
		data.Matches,
		data.Screenshot,
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

func AddTarget(url string, source string) error {
	query := `INSERT INTO targets (url, source, last_status) VALUES ($1, $2, 'Pending') ON CONFLICT (url) DO NOTHING`
	_, err := DB.Exec(query, url, source)
	return err
}

func DeleteTarget(id int) error {
	query := `DELETE FROM targets WHERE id = $1`
	_, err := DB.Exec(query, id)
	return err
}

func GetTargetByID(id int) (models.Target, error) {
	query := `
    SELECT id, url, source, created_at, COALESCE(last_status, 'Pending'), COALESCE(last_scanned_at, created_at)
    FROM targets WHERE id = $1`

	var t models.Target
	err := DB.QueryRow(query, id).Scan(&t.ID, &t.URL, &t.Source, &t.CreatedAt, &t.LastStatus, &t.LastScannedAt)
	return t, err
}

func GetAllTargets() ([]models.Target, error) {

	query := `
    SELECT id, url, source, created_at, COALESCE(last_status, 'Pending'), COALESCE(last_scanned_at, created_at)
    FROM targets 
    ORDER BY created_at DESC`

	rows, err := DB.Query(query)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []models.Target
	for rows.Next() {
		var t models.Target
		if err := rows.Scan(&t.ID, &t.URL, &t.Source, &t.CreatedAt, &t.LastStatus, &t.LastScannedAt); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}

	return targets, nil
}

func SearchContent(queryText string) ([]models.DarkWebContent, error) {

	querySQL := `
    SELECT id, source_name, source_url, content, title, published_date, criticality_score, category, matches 
    FROM dark_web_contents 
    WHERE 
        source_url ILIKE $1 OR 
        title ILIKE $1 OR 
        content ILIKE $1 OR 
        matches ILIKE $1
    ORDER BY published_date DESC`

	searchTerm := "%" + queryText + "%"

	rows, err := DB.Query(querySQL, searchTerm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []models.DarkWebContent
	for rows.Next() {
		var c models.DarkWebContent
		if err := rows.Scan(&c.ID, &c.SourceName, &c.SourceURL, &c.Content, &c.Title,
			&c.PublishedDate, &c.CriticalityScore, &c.Category, &c.Matches); err != nil {
			return nil, err
		}
		contents = append(contents, c)
	}
	return contents, nil
}

func AddLinkRelationship(source string, target string) error {
	if source == target {
		return nil
	}
	query := `INSERT INTO link_relationships (source_url, target_url) VALUES ($1, $2) ON CONFLICT (source_url, target_url) DO NOTHING`
	_, err := DB.Exec(query, source, target)
	return err
}

func GetGraphData() ([]models.GraphNode, []models.GraphEdge, error) {
	nodeQuery := `
        SELECT DISTINCT ON (source_url) source_url, title, criticality_score, category 
        FROM dark_web_contents 
        ORDER BY source_url, published_date DESC`

	rows, err := DB.Query(nodeQuery)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var nodes []models.GraphNode
	for rows.Next() {
		var n models.GraphNode
		if err := rows.Scan(&n.ID, &n.Label, &n.Value, &n.Group); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}

	edgeQuery := `SELECT source_url, target_url FROM link_relationships`
	edgeRows, err := DB.Query(edgeQuery)
	if err != nil {
		return nil, nil, err
	}
	defer edgeRows.Close()

	var edges []models.GraphEdge
	for edgeRows.Next() {
		var e models.GraphEdge
		if err := edgeRows.Scan(&e.From, &e.To); err != nil {
			continue
		}
		edges = append(edges, e)
	}

	return nodes, edges, nil
}
