package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
