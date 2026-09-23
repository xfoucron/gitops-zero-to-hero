package models

import "time"

type ClickEvent struct {
	LinkID    int64     `json:"link_id"`
	UserAgent string    `json:"user_agent"`
	ClickedAt time.Time `json:"clicked_at"`
}
