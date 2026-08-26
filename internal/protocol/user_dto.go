package protocol

import (
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type CreateUserRequest struct {
	Name     string          `json:"name"`
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Role     models.UserRole `json:"role"`
}

type UserResponse struct {
	ID    uuid.UUID       `json:"uuid"`
	Name  string          `json:"name"`
	Email string          `json:"email"`
	Role  models.UserRole `json:"role"`
}

type DriverResponse struct {
	UserResponse
	Vehicle *models.Vehicle `json:"vehicle,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
