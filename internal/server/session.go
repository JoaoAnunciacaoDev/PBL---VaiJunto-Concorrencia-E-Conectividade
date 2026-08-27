package server

import (
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type Session struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Role     models.UserRole
	UserName string
}

func NewSession() *Session {
	return &Session{ID: uuid.New()}
}

func (s *Session) IsAuthenticated() bool {
	return s.UserID != uuid.Nil
}

func (s *Session) Clear() {
	s.UserID = uuid.Nil
	s.Role = ""
	s.UserName = ""
}
