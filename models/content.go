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
}
