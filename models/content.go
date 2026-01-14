package models

import (
	"time"
)

type DarkWebContent struct {
	ID               int       `json:"id"`
	SourceName       string    `json:"source_name"`       //
	SourceURL        string    `json:"source_url"`        //
	Content          string    `json:"content"`           //
	Title            string    `json:"title"`             //
	PublishedDate    time.Time `json:"published_date"`    //
	CriticalityScore int       `json:"criticality_score"` //
	Category         string    `json:"category"`          //
	CreatedAt        time.Time `json:"created_at"`        //
	Matches          string    `json:"matches"`           //
	Screenshot       string    `json:"screenshot"`        //
}

type Target struct {
	ID            int       `json:"id"`
	URL           string    `json:"url"`
	Source        string    `json:"source"` // yaml, database, etc.
	CreatedAt     time.Time `json:"created_at"`
	LastStatus    string    `json:"last_status"`     // Ex: "Scanning", "Online", "Failed"
	LastScannedAt time.Time `json:"last_scanned_at"` // Ex: 2026-01-10...
}
