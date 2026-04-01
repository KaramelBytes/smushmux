package project

import "time"

// Document holds metadata and cached content for a project document.
type Document struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Tokens      int       `json:"tokens"`
	AddedAt     time.Time `json:"added_at"`
}
