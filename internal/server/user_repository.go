package server

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type storedUser struct {
	ID           uuid.UUID       `json:"uuid"`
	Name         string          `json:"name"`
	Email        string          `json:"email"`
	PasswordHash string          `json:"password_hash"`
	Role         models.UserRole `json:"role"`
}

func (r *Repository) loadUsers() error {
	var users []storedUser
	if err := readJSONFile(r.usersPath, &users); err != nil {
		return err
	}

	for _, stored := range users {
		if stored.ID == uuid.Nil || stored.Email == "" || stored.PasswordHash == "" {
			return errors.New("arquivo de usuários contém um registro inválido")
		}

		email := strings.ToLower(strings.TrimSpace(stored.Email))
		if _, exists := r.users[email]; exists {
			return errors.New("arquivo de usuários contém e-mails duplicados")
		}

		r.users[email] = &models.User{
			ID:           stored.ID,
			Name:         stored.Name,
			Email:        email,
			PasswordHash: stored.PasswordHash,
			Role:         stored.Role,
		}
	}

	return nil
}

func (r *Repository) saveUsersLocked() error {
	users := make([]storedUser, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, storedUser{
			ID:           user.ID,
			Name:         user.Name,
			Email:        user.Email,
			PasswordHash: user.PasswordHash,
			Role:         user.Role,
		})
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Email < users[j].Email
	})

	return writeJSONFileAtomic(r.usersPath, ".users-*.tmp", users)
}

func (r *Repository) SaveUser(user *models.User) error {
	if user == nil {
		return errors.New("usuário não pode ser nulo")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))
	user.Email = normalizedEmail

	if _, exists := r.users[normalizedEmail]; exists {
		return errors.New("Usuário já existe.")
	}

	r.users[normalizedEmail] = user
	if err := r.saveUsersLocked(); err != nil {
		delete(r.users, normalizedEmail)
		return fmt.Errorf("salvar usuário: %w", err)
	}

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

func (r *Repository) GetUserByID(id uuid.UUID) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.userByIDLocked(id)
	if !exists {
		return nil, errors.New("usuário não encontrado")
	}

	return user, nil
}

func (r *Repository) userByIDLocked(id uuid.UUID) (*models.User, bool) {
	for _, user := range r.users {
		if user.ID == id {
			return user, true
		}
	}

	return nil, false
}
