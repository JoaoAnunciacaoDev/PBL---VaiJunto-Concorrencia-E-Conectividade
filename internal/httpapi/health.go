package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type healthResponse struct {
	CompanyID    string `json:"company_id"`
	Status       string `json:"status"`
	LamportClock uint64 `json:"lamport_clock"`
}

func (s *HTTPServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{
		CompanyID:    s.companyID,
		Status:       "ok",
		LamportClock: s.clock.Value(),
	})
}
