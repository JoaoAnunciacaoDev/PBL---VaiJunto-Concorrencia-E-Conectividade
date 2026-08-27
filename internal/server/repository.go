package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type Repository struct {
	mu        sync.RWMutex
	users     map[string]*models.User
	usersPath string
}

type storedUser struct {
	ID           uuid.UUID       `json:"uuid"`
	Name         string          `json:"name"`
	Email        string          `json:"email"`
	PasswordHash string          `json:"password_hash"`
	Role         models.UserRole `json:"role"`
}

func NewRepository(usersPath string) (*Repository, error) {
	repository := &Repository{
		users:     make(map[string]*models.User),
		usersPath: usersPath,
	}

	if err := repository.loadUsers(); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *Repository) loadUsers() error {
	content, err := os.ReadFile(r.usersPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ler arquivo de usuários: %w", err)
	}
	if len(content) == 0 {
		return nil
	}

	var users []storedUser
	if err := json.Unmarshal(content, &users); err != nil {
		return fmt.Errorf("interpretar arquivo de usuários: %w", err)
	}

	for _, stored := range users {
		if stored.Email == "" || stored.PasswordHash == "" {
			return errors.New("arquivo de usuários contém um registro inválido")
		}

		email := strings.ToLower(strings.TrimSpace(stored.Email))
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

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar usuários: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(r.usersPath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("criar diretório de dados: %w", err)
	}

	temporaryFile, err := os.CreateTemp(directory, ".users-*.tmp")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporaryFile.Write(data); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("gravar arquivo temporário: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("fechar arquivo temporário: %w", err)
	}
	if err := os.Rename(temporaryPath, r.usersPath); err != nil {
		return fmt.Errorf("substituir arquivo de usuários: %w", err)
	}

	return nil
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

	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}

	return nil, errors.New("usuário não encontrado")
}
