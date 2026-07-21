package dto

type MessageResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Status  int    `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}
