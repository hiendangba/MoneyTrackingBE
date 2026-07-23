package dto

type UpdateMenuRequest struct {
	Code      *string `json:"code"`
	Name      *string `json:"name"`
	Route     *string `json:"route"`
	Icon      *string `json:"icon"`
	ParentID  *string `json:"parent_id"`
	SortOrder *int    `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
}
