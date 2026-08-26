package server

import (
	"errors"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"strings"
	"sync"
)

type Repository struct {
	mu    sync.RWMutex
	users map[string]*models.User
}

func NewRepository() *Repository {
	return &Repository{
		users: make(map[string]*models.User),
	}
}

func (r *Repository) SaveUser(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))
	user.Email = normalizedEmail

	if _, exists := r.users[normalizedEmail]; exists {
		return errors.New("Usuário já existe.")
	}

	r.users[normalizedEmail] = user

	return nil
}

func (r *Repository) GetUserByEmail(email string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	user, exists := r.users[normalizedEmail]

	if !exists {
		return nil, errors.New("Usuário não encontrado.")
	}

	return user, nil
}
