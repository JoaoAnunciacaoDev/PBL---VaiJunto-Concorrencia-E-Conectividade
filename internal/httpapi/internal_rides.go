package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *HTTPServer) handleGetRides(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parâmetro 'date' é obrigatório (formato YYYY-MM-DD)"})
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		date, err = time.Parse(time.RFC3339, dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "data inválida: use YYYY-MM-DD ou RFC3339"})
			return
		}
	}

	rides, err := s.repo.GetActiveRidesByDate(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "não foi possível buscar caronas"})
		return
	}

	s.clock.Tick()

	c.JSON(http.StatusOK, rides)
}

func (s *HTTPServer) handleGetRide(c *gin.Context) {
    rideID, err := uuid.Parse(c.Param("rideID"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "ID de carona inválido"})
        return
    }

    ride, err := s.repo.GetRideByID(rideID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "carona não encontrada"})
        return
    }

    c.JSON(http.StatusOK, ride)
}