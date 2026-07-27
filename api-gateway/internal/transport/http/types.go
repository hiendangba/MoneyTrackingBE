package httptransport

import "time"

type ErrorResponse struct {
	Status  int    `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type TokenResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresIn        int64  `json:"expires_in,omitempty"`
	RefreshExpiresIn int64  `json:"refresh_expires_in,omitempty"`
}

type CSRFResponse struct {
	CSRFToken string `json:"csrf_token"`
}

type AuthClaims struct {
	UserID         string
	RoleID         string
	RoleCode       string
	SessionID      string
	SessionVersion int64
}

type RegisterRequest struct {
	Fullname string `json:"fullname"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type VerifyOTPRequest struct {
	Email   string `json:"email"`
	OTP     string `json:"otp"`
	Purpose string `json:"purpose"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Email       string `json:"email"`
	OTP         string `json:"otp"`
	NewPassword string `json:"new_password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type MeResponse struct {
	ID       string `json:"id"`
	Fullname string `json:"fullname"`
	Email    string `json:"email"`
	RoleID   string `json:"role_id"`
	RoleCode string `json:"role_code"`
}

type Menu struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Route     string  `json:"route"`
	Icon      *string `json:"icon,omitempty"`
	ParentID  *string `json:"parent_id,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsActive  bool    `json:"is_active"`
}

type MenuNode struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Route     string     `json:"route"`
	Icon      *string    `json:"icon,omitempty"`
	SortOrder int        `json:"sort_order"`
	Children  []MenuNode `json:"children"`
}

type CreateMenuRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Route     string `json:"route"`
	Icon      string `json:"icon"`
	ParentID  string `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
	IsActive  *bool  `json:"is_active"`
}

type UpdateMenuRequest struct {
	Code      *string `json:"code"`
	Name      *string `json:"name"`
	Route     *string `json:"route"`
	Icon      *string `json:"icon"`
	ParentID  *string `json:"parent_id"`
	SortOrder *int    `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
}

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type GroupMember struct {
	GroupID   string    `json:"group_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GroupInvitation struct {
	ID            string     `json:"id"`
	GroupID       string     `json:"group_id"`
	InvitedUserID string     `json:"invited_user_id"`
	InvitedBy     string     `json:"invited_by"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	Token         string     `json:"token"`
	ExpiredAt     *time.Time `json:"expired_at,omitempty"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

type AddMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type InviteMemberRequest struct {
	InvitedUserID string     `json:"invited_user_id"`
	Role          string     `json:"role"`
	ExpiredAt     *time.Time `json:"expired_at,omitempty"`
}

type RespondInvitationRequest struct {
	Accepted bool `json:"accepted"`
}

type Category struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserSnapshot struct {
	UserID   string `json:"user_id"`
	Fullname string `json:"fullname"`
	Email    string `json:"email"`
}

type PersonalTransaction struct {
	ID              string       `json:"id"`
	User            UserSnapshot `json:"user"`
	Category        *Category    `json:"category,omitempty"`
	Type            string       `json:"type"`
	Title           string       `json:"title"`
	Amount          string       `json:"amount"`
	Currency        string       `json:"currency"`
	TransactionDate time.Time    `json:"transaction_date"`
	Note            *string      `json:"note,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type GroupPayment struct {
	ID        string       `json:"id"`
	User      UserSnapshot `json:"user"`
	Amount    string       `json:"amount"`
	Note      *string      `json:"note,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

type GroupSplit struct {
	ID          string       `json:"id"`
	User        UserSnapshot `json:"user"`
	SplitType   string       `json:"split_type"`
	SplitValue  string       `json:"split_value"`
	ShareAmount string       `json:"share_amount"`
	CreatedAt   time.Time    `json:"created_at"`
}

type GroupTransaction struct {
	ID              string         `json:"id"`
	GroupID         string         `json:"group_id"`
	GroupName       string         `json:"group_name"`
	Category        *Category      `json:"category,omitempty"`
	Type            string         `json:"type"`
	Title           string         `json:"title"`
	Amount          string         `json:"amount"`
	Currency        string         `json:"currency"`
	TransactionDate time.Time      `json:"transaction_date"`
	Note            *string        `json:"note,omitempty"`
	CreatedBy       UserSnapshot   `json:"created_by"`
	Payments        []GroupPayment `json:"payments"`
	Splits          []GroupSplit   `json:"splits"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type Settlement struct {
	ID        string       `json:"id"`
	GroupID   string       `json:"group_id"`
	GroupName string       `json:"group_name"`
	FromUser  UserSnapshot `json:"from_user"`
	ToUser    UserSnapshot `json:"to_user"`
	Amount    string       `json:"amount"`
	Currency  string       `json:"currency"`
	SettledAt time.Time    `json:"settled_at"`
	Note      *string      `json:"note,omitempty"`
	CreatedBy UserSnapshot `json:"created_by"`
	CreatedAt time.Time    `json:"created_at"`
}

type MemberBalance struct {
	User                UserSnapshot `json:"user"`
	Paid                string       `json:"paid"`
	Share               string       `json:"share"`
	SettlementsSent     string       `json:"settlements_sent"`
	SettlementsReceived string       `json:"settlements_received"`
	NetBalance          string       `json:"net_balance"`
}

type SettlementSuggestion struct {
	FromUser UserSnapshot `json:"from_user"`
	ToUser   UserSnapshot `json:"to_user"`
	Amount   string       `json:"amount"`
	Currency string       `json:"currency"`
}

type GroupBalancesResponse struct {
	Balances    []MemberBalance        `json:"balances"`
	Suggestions []SettlementSuggestion `json:"suggestions"`
}

type PaginatedPersonalTransactions struct {
	Items []PersonalTransaction `json:"items"`
	Total int64                 `json:"total"`
}

type PaginatedGroupTransactions struct {
	Items []GroupTransaction `json:"items"`
	Total int64              `json:"total"`
}

type PaginatedSettlements struct {
	Items []Settlement `json:"items"`
	Total int64        `json:"total"`
}

type CreateCategoryRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreatePersonalTransactionRequest struct {
	CategoryID      string    `json:"category_id"`
	Type            string    `json:"type"`
	Title           string    `json:"title"`
	Amount          string    `json:"amount"`
	Currency        string    `json:"currency"`
	TransactionDate time.Time `json:"transaction_date"`
	Note            string    `json:"note"`
}

type PaymentInput struct {
	UserID string `json:"user_id"`
	Amount string `json:"amount"`
	Note   string `json:"note"`
}

type SplitInput struct {
	UserID     string `json:"user_id"`
	SplitType  string `json:"split_type"`
	SplitValue string `json:"split_value"`
}

type CreateGroupTransactionRequest struct {
	CategoryID      string         `json:"category_id"`
	Type            string         `json:"type"`
	Title           string         `json:"title"`
	Amount          string         `json:"amount"`
	Currency        string         `json:"currency"`
	TransactionDate time.Time      `json:"transaction_date"`
	Note            string         `json:"note"`
	Payments        []PaymentInput `json:"payments"`
	Splits          []SplitInput   `json:"splits"`
}

type CreateSettlementRequest struct {
	FromUserID string    `json:"from_user_id"`
	ToUserID   string    `json:"to_user_id"`
	Amount     string    `json:"amount"`
	Currency   string    `json:"currency"`
	SettledAt  time.Time `json:"settled_at"`
	Note       string    `json:"note"`
}
