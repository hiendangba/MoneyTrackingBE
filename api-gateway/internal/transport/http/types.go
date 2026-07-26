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
