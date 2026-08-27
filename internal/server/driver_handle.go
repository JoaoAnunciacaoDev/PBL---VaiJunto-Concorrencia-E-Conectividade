package server

import (
	"encoding/json"
	"strings"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func (s *Server) handleRegisterVehicle(payload json.RawMessage, session *Session) protocol.Response {
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
	if !user.IsDriver() {
		return protocol.Response{
			Success: "error",
			Message: "Apenas motoristas podem cadastrar veículos.",
		}
	}

	var dto protocol.CreateVehicleRequest
	if err := json.Unmarshal(payload, &dto); err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Requisição inválida: " + err.Error(),
		}
	}

	vehicle := models.Vehicle{
		Plate:        strings.ToUpper(strings.TrimSpace(dto.Plate)),
		Model:        strings.TrimSpace(dto.Model),
		Color:        strings.TrimSpace(dto.Color),
		SeatCapacity: dto.SeatCapacity,
	}
	if vehicle.Plate == "" || vehicle.Model == "" || vehicle.Color == "" || vehicle.SeatCapacity <= 0 {
		return protocol.Response{
			Success: "error",
			Message: "Placa, modelo, cor e capacidade maior que zero são obrigatórios.",
		}
	}

	driver, err := s.repository.GetDriverByUserID(user.ID)
	if err != nil {
		driver = &models.Driver{User: *user}
	}
	if driver.HasVehicle() {
		return protocol.Response{
			Success: "error",
			Message: "O motorista já possui um veículo cadastrado.",
		}
	}

	driver.RegisterVehicle(vehicle)
	if err := s.repository.SaveDriver(driver); err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Não foi possível cadastrar o veículo.",
		}
	}

	data, err := json.Marshal(driver.Vehicle)
	if err != nil {
		return protocol.Response{
			Success: "error",
			Message: "Não foi possível processar a resposta.",
		}
	}

	return protocol.Response{
		Success: "success",
		Message: "Veículo cadastrado com sucesso.",
		Payload: data,
	}
}
