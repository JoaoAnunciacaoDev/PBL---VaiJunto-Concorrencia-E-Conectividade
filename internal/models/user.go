package models

import "github.com/google/uuid"

type UserRole string

const (
	RolePassenger UserRole = "PASSENGER"
	RoleDriver    UserRole = "DRIVER"
)

type User struct {
	ID           uuid.UUID `json:"uuid"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         UserRole  `json:"role"`
}

func (u *User) IsDriver() bool {
	return u.Role == RoleDriver
}
