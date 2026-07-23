package dto

type MenuResponse struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Route     string  `json:"route"`
	Icon      *string `json:"icon,omitempty"`
	ParentID  *string `json:"parent_id,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsActive  bool    `json:"is_active"`
}
