package auth

// User is the authenticated principal attached to requests.
type User struct {
	ID       string
	Email    string
	Name     string
	Role     string // "partner" | "staff"
	IsActive bool

	Partner *PartnerInfo
	Staff   []StaffRole
}

// PartnerInfo carries the partner profile of any user with a partner identity.
type PartnerInfo struct {
	PartnerID      string
	IsApproved     bool
	IsBlocked      bool
	ReferralCode   string
	TelegramUserID *int64
}

// StaffRole is one role assignment of a staff user.
type StaffRole struct {
	Role      string
	ProjectID *string
}
