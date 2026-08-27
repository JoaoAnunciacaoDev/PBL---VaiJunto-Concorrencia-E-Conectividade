package server

import (
	"encoding/json"
	"strings"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleRegisterUser(payload json.RawMessage) protocol.Response {
	var dto protocol.CreateUserRequest

	if err := json.Unmarshal(payload, &dto); err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Requisição inválida: " + err.Error(),
		}
	}

	name := strings.TrimSpace(dto.Name)
	email := strings.ToLower(strings.TrimSpace(dto.Email))

	if name == "" {
		return protocol.Response{
			Success: "error",
			Message: "Nome é obrigatório.",
		}
	}
	if !isValidEmail(email) {
		return protocol.Response{
			Success: "error",
			Message: "E-mail inválido.",
		}
	}
	if !isValidPassword(dto.Password) {
		return protocol.Response{
			Success: "error",
			Message: "A senha deve ter ao menos 8 caracteres, uma letra, um número e um caractere especial.",
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(dto.Password), bcrypt.DefaultCost)
	if err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Não foi possível cadastrar o usuário.",
		}
	}

	user := models.User{
		ID:           uuid.New(),
		Name:         name,
		Email:        email,
		PasswordHash: string(passwordHash),
		Role:         dto.Role,
	}

	if err := s.repository.SaveUser(&user); err != nil {
		return protocol.Response{
			Success: "error",
			Message: err.Error(),
		}
	}

	return userResponse(user, "Usuário cadastrado com sucesso")
}

func (s *Server) handleLogin(payload json.RawMessage, session *Session) protocol.Response {
	var dto protocol.LoginRequest

	if err := json.Unmarshal(payload, &dto); err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Requisição inválida: " + err.Error(),
		}
	}

	email := strings.ToLower(strings.TrimSpace(dto.Email))
	if email == "" || dto.Password == "" {
		return protocol.Response{
			Success: "error",
			Message: "E-mail e senha são obrigatórios.",
		}
	}

	user, err := s.repository.GetUserByEmail(email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(dto.Password)) != nil {
		return protocol.Response{
			Success: "error",
			Message: "E-mail ou senha inválidos.",
		}
	}

	session.UserID = user.ID
	session.Role = user.Role

	return userResponse(*user, "Login bem-sucedido")
}

func (s *Server) handleGetMyProfile(session *Session) protocol.Response {
	if !session.IsAuthenticated() {
		return protocol.Response{
			Success: "error",
			Message: "Autenticação necessária.",
		}
	}

	user, err := s.repository.GetUserByID(session.UserID)
	if err != nil {
		session.Clear()
		return protocol.Response{
			Success: "error",
			Message: "Sessão inválida. Faça login novamente.",
		}
	}

	return userResponse(*user, "Perfil obtido com sucesso")
}

func (s *Server) handleLogout(session *Session) protocol.Response {
	if !session.IsAuthenticated() {
		return protocol.Response{
			Success: "error",
			Message: "Nenhum usuário está autenticado.",
		}
	}

	session.Clear()
	return protocol.Response{
		Success: "success",
		Message: "Logout realizado com sucesso.",
	}
}

func userResponse(user models.User, message string) protocol.Response {
	data, err := json.Marshal(protocol.UserResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Role:  user.Role,
	})
	if err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Não foi possível processar a resposta.",
		}
	}

	return protocol.Response{
		Success: "success",
		Message: message,
		Payload: data,
	}
}
