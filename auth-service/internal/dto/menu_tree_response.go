package dto

type MenuTreeResponse struct {
	ID        string             `json:"id"`
	Code      string             `json:"code"`
	Name      string             `json:"name"`
	Route     string             `json:"route"`
	Icon      *string            `json:"icon,omitempty"`
	SortOrder int                `json:"sort_order"`
	Children  []MenuTreeResponse `json:"children"`
}
