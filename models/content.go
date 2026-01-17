package models

import (
	"time"
)

type DarkWebContent struct {
	ID               int               `json:"id"`
	SourceName       string            `json:"source_name"`       //
	SourceURL        string            `json:"source_url"`        //
	Content          string            `json:"content"`           //
	Title            string            `json:"title"`             //
	PublishedDate    time.Time         `json:"published_date"`    //
	CriticalityScore int               `json:"criticality_score"` //
	Category         string            `json:"category"`          //
	CreatedAt        time.Time         `json:"created_at"`        //
	Matches          string            `json:"matches"`           //
	Screenshot       string            `json:"screenshot"`        //
	Entities         []ExtractedEntity `json:"entities"`          //
}

type Target struct {
	ID            int       `json:"id"`
	URL           string    `json:"url"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
	LastStatus    string    `json:"last_status"`
	LastScannedAt time.Time `json:"last_scanned_at"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
	Group string `json:"group"`
}

type GraphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Color  string `json:"color"`
	Dashes bool   `json:"dashes"`
	Label  string `json:"label"`
}

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
}

type ExtractedEntity struct {
	Type  string
	Value string
}
