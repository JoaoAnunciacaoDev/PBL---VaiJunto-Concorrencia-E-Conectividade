package server

import (
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type Session struct {
	UserID uuid.UUID
	Role   models.UserRole
}

func (s *Session) IsAuthenticated() bool {
	return s.UserID != uuid.Nil
}

func (s *Session) Clear() {
	s.UserID = uuid.Nil
	s.Role = ""
}
