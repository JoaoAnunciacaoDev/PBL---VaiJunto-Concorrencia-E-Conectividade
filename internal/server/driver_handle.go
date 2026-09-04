package server

import (
	"encoding/json"
	"strings"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func (s *Server) handleRegisterVehicle(payload json.RawMessage, session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, false)

	if !ok {
		return response
	}

	if driver.HasVehicle() {
		return protocol.Response{Success: "error", Message: "O motorista já possui um veículo cadastrado."}
	}

	vehicle, response, ok := vehicleFromPayload(payload)

	if !ok {
		return response
	}

	driver.RegisterVehicle(vehicle)

	if err := s.repository.SaveDriver(driver); err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível cadastrar o veículo."}
	}

	return vehicleResponse(*driver.Vehicle, "Veículo cadastrado com sucesso.")
}

func (s *Server) handleGetMyVehicle(session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, true)

	if !ok {
		return response
	}

	return vehicleResponse(*driver.Vehicle, "Veículo obtido com sucesso.")
}

func (s *Server) handleUpdateVehicle(payload json.RawMessage, session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, true)

	if !ok {
		return response
	}

	vehicle, response, ok := vehicleFromPayload(payload)

	if !ok {
		return response
	}

	if err := driver.UpdateVehicle(vehicle.Plate, vehicle.Model, vehicle.Color, vehicle.SeatCapacity); err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível atualizar o veículo."}
	}

	if err := s.repository.SaveDriver(driver); err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível atualizar o veículo."}
	}

	return vehicleResponse(*driver.Vehicle, "Veículo atualizado com sucesso.")
}

func (s *Server) handleRemoveVehicle(session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, true)

	if !ok {
		return response
	}

	driver.RemoveVehicle()

	if err := s.repository.SaveDriver(driver); err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível remover o veículo."}
	}

	return protocol.Response{Success: "success", Message: "Veículo removido com sucesso."}
}

func (s *Server) driverForSession(session *Session, requiresVehicle bool) (*models.Driver, protocol.Response, bool) {
	if !session.IsAuthenticated() {
		return nil, protocol.Response{Success: "error", Message: "Autenticação necessária."}, false
	}

	user, err := s.repository.GetUserByID(session.UserID)

	if err != nil {
		session.Clear()
		return nil, protocol.Response{Success: "error", Message: "Sessão inválida. Faça login novamente."}, false
	}

	if !user.IsDriver() {
		return nil, protocol.Response{Success: "error", Message: "Apenas motoristas podem executar esta operação."}, false
	}

	driver, err := s.repository.GetDriverByUserID(user.ID)

	if err != nil {
		if requiresVehicle {
			return nil, protocol.Response{Success: "error", Message: "Nenhum veículo cadastrado."}, false
		}
		return &models.Driver{User: *user}, protocol.Response{}, true
	}

	if requiresVehicle && !driver.HasVehicle() {
		return nil, protocol.Response{Success: "error", Message: "Nenhum veículo cadastrado."}, false
	}

	return driver, protocol.Response{}, true
}

func vehicleFromPayload(payload json.RawMessage) (models.Vehicle, protocol.Response, bool) {
	var dto protocol.CreateVehicleRequest

	if err := json.Unmarshal(payload, &dto); err != nil {
		return models.Vehicle{}, protocol.Response{Success: "error", Message: "Requisição inválida: " + err.Error()}, false
	}

	vehicle := models.Vehicle{
		Plate:        strings.ToUpper(strings.TrimSpace(dto.Plate)),
		Model:        strings.TrimSpace(dto.Model),
		Color:        strings.TrimSpace(dto.Color),
		SeatCapacity: dto.SeatCapacity,
	}

	if vehicle.Plate == "" || vehicle.Model == "" || vehicle.Color == "" || vehicle.SeatCapacity <= 0 {
		return models.Vehicle{}, protocol.Response{
				Success: "error",
				Message: "Placa, modelo, cor e capacidade maior que zero são obrigatórios."},
			false
	}

	return vehicle, protocol.Response{}, true
}

func vehicleResponse(vehicle models.Vehicle, message string) protocol.Response {
	data, err := json.Marshal(vehicle)

	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível processar a resposta."}
	}

	return protocol.Response{Success: "success", Message: message, Payload: data}
}
