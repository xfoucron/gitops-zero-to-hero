package models

import "time"

type Link struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	TargetURL string    `json:"target_url"`
	CreatedAt time.Time `json:"created_at"`
}
