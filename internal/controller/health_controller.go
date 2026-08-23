package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"gorm.io/gorm"
)

type HealthResponse struct {
	Status      string `json:"status"`
	Database    string `json:"database"`
	Environment string `json:"environment,omitempty"`
	Timestamp   string `json:"timestamp"`
}

type HealthController struct {
	db *gorm.DB
}

func NewHealthController(db *gorm.DB) *HealthController {
	return &HealthController{db: db}
}

func (c *HealthController) HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method != http.MethodGet {
		return logAndBuildError(req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil), nil
	}

	dbStatus := "connected"
	statusCode := http.StatusOK
	status := "ok"

	if c.db != nil {
		sqlDB, err := c.db.DB()
		if err != nil || sqlDB.PingContext(ctx) != nil {
			dbStatus = "disconnected"
			statusCode = http.StatusServiceUnavailable
			status = "degraded"
		}
	} else {
		dbStatus = "disconnected"
		statusCode = http.StatusServiceUnavailable
		status = "degraded"
	}

	response := HealthResponse{
		Status:    status,
		Database:  dbStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return buildSuccessResponse(req, statusCode, response)
}
