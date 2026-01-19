package database

import (
	"interactive-scraper/models"
	"log"
	"time"
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

	entityQuery := `SELECT entity_type, entity_value FROM entities WHERE content_id = $1`
	rows, err := DB.Query(entityQuery, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e models.ExtractedEntity
			if err := rows.Scan(&e.Type, &e.Value); err == nil {
				c.Entities = append(c.Entities, e)
			}
		}
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

		if len(n.Label) > 20 {
			n.Label = n.Label[:20] + "..."
		}
		nodes = append(nodes, n)
	}

	var edges []models.GraphEdge

	linkQuery := `SELECT source_url, target_url FROM link_relationships`
	linkRows, err := DB.Query(linkQuery)
	if err == nil {
		defer linkRows.Close()
		for linkRows.Next() {
			var e models.GraphEdge
			linkRows.Scan(&e.From, &e.To)
			e.Color = "#0f62fe"
			e.Dashes = false
			edges = append(edges, e)
		}
	}

	sharedQuery := `
		SELECT t1.source_url, t2.source_url, e1.entity_type
		FROM entities e1
		JOIN entities e2 ON e1.entity_value = e2.entity_value AND e1.id != e2.id
		JOIN dark_web_contents t1 ON e1.content_id = t1.id
		JOIN dark_web_contents t2 ON e2.content_id = t2.id
		WHERE t1.source_url < t2.source_url -- Mükerrer (A-B ve B-A) kayıtları önler
		GROUP BY t1.source_url, t2.source_url, e1.entity_type
	`

	sharedRows, err := DB.Query(sharedQuery)
	if err == nil {
		defer sharedRows.Close()
		for sharedRows.Next() {
			var from, to, eType string
			sharedRows.Scan(&from, &to, &eType)

			edges = append(edges, models.GraphEdge{
				From:   from,
				To:     to,
				Color:  "#da1e28",
				Dashes: true,
				Label:  eType,
			})
		}
	}

	return nodes, edges, nil
}

func SaveEntities(contentID int, entities []models.ExtractedEntity) error {
	if len(entities) == 0 {
		return nil
	}

	query := `INSERT INTO entities (content_id, entity_type, entity_value) VALUES ($1, $2, $3)`

	for _, e := range entities {
		_, err := DB.Exec(query, contentID, e.Type, e.Value)
		if err != nil {
			log.Printf("Entity save error (%s): %v", e.Type, err)

		}
	}
	return nil
}

// GetPreviousScan returns the latest scan for a given URL before current time
func GetPreviousScan(url string, beforeTime time.Time) (*models.DarkWebContent, error) {
	query := `
    SELECT id, source_name, source_url, content, title, published_date, criticality_score, category, matches, COALESCE(screenshot, '')
    FROM dark_web_contents
    WHERE source_url = $1 AND published_date < $2
    ORDER BY published_date DESC
    LIMIT 1`

	var c models.DarkWebContent
	err := DB.QueryRow(query, url, beforeTime).Scan(
		&c.ID, &c.SourceName, &c.SourceURL, &c.Content, &c.Title,
		&c.PublishedDate, &c.CriticalityScore, &c.Category, &c.Matches, &c.Screenshot,
	)

	if err != nil {
		return nil, err
	}

	// Load entities for previous scan
	entityQuery := `SELECT entity_type, entity_value FROM entities WHERE content_id = $1`
	rows, err := DB.Query(entityQuery, c.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e models.ExtractedEntity
			if err := rows.Scan(&e.Type, &e.Value); err == nil {
				c.Entities = append(c.Entities, e)
			}
		}
	}

	return &c, nil
}

// DetectNewEntities compares current entities with previous scan and returns only new ones
func DetectNewEntities(currentEntities []models.ExtractedEntity, previousEntities []models.ExtractedEntity) []models.EntityChange {
	var newEntities []models.EntityChange

	// Build a map of previous entities for fast lookup
	prevMap := make(map[string]bool)
	for _, e := range previousEntities {
		key := e.Type + ":" + e.Value
		prevMap[key] = true
	}

	// Check which current entities are new
	for _, curr := range currentEntities {
		key := curr.Type + ":" + curr.Value
		if !prevMap[key] {
			newEntities = append(newEntities, models.EntityChange{
				Type:      curr.Type,
				Value:     curr.Value,
				IsNew:     true,
				ScannedAt: time.Now(),
			})
		}
	}

	return newEntities
}

// GetRiskLevel determines risk level based on score
func GetRiskLevel(score int) string {
	if score >= 8 {
		return "CRITICAL"
	} else if score >= 5 {
		return "HIGH"
	} else if score >= 3 {
		return "MEDIUM"
	}
	return "LOW"
}
