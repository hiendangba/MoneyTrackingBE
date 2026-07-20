package dto

type MeResponse struct {
	ID       string `json:"id"`
	Fullname string `json:"fullname"`
	Email    string `json:"email"`
	RoleID   string `json:"role_id"`
}
