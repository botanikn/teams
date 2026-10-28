package domain

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type Team struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedBy int64     `json:"-"`
	OwnerName string    `json:"owner_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
