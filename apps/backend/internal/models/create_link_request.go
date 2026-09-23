package models

type CreateLinkRequest struct {
	Slug      string `json:"slug"`
	TargetURL string `json:"target_url"`
}
