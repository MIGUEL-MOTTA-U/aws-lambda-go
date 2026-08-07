package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/service"
)

// VisitService is the subset of service.VisitService used by the
// controller. Defined as an interface here to keep the controller
// package free of imports from internal/service beyond what the other
// controllers already pull in.
type VisitService interface {
	RecordEvent(ctx context.Context, req service.VisitRecordRequest) (model.Visit, error)
}

type VisitController struct {
	service VisitService
}

func NewVisitController(svc VisitService) *VisitController {
	return &VisitController{service: svc}
}

// HandleRequest routes POST /visits. The route is public (no Cognito
// guard required in main.go) because visitors are anonymous.
func (c VisitController) HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := req.RequestContext.HTTP.Method
	path := normalizePath(req.RawPath)

	if path != "/visits" {
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "route not found", nil), nil
	}

	switch method {
	case http.MethodPost:
		return c.recordVisit(ctx, req)
	default:
		return logAndBuildError(req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil), nil
	}
}

func (c VisitController) recordVisit(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var payload struct {
		VisitorID  string  `json:"visitor_id"`
		ListingID  string  `json:"listing_id"`
		EventType  string  `json:"event_type"`
		Source     string  `json:"source"`
		DurationMs *int    `json:"duration_ms"`
	}

	if strings.TrimSpace(req.Body) == "" {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "request body is required", nil), nil
	}
	decoder := json.NewDecoder(strings.NewReader(req.Body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body", err), nil
	}

	// Source is optional in the payload but the service defaults empty
	// strings to model.VisitSourcePublic. Keep the field empty here so
	// that logic stays in one place.
	visitReq := service.VisitRecordRequest{
		VisitorID:  payload.VisitorID,
		ListingID:  payload.ListingID,
		EventType:  model.VisitEventType(payload.EventType),
		Source:     model.VisitSource(payload.Source),
		DurationMs: payload.DurationMs,
		UserAgent:  headerValue(req.Headers, "User-Agent"),
	}

	if _, err := c.service.RecordEvent(ctx, visitReq); err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusNoContent, nil)
}

func (c VisitController) errorToResponse(req events.APIGatewayV2HTTPRequest, err error) events.APIGatewayV2HTTPResponse {
	switch {
	case errors.Is(err, service.ErrVisitInvalid):
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
	case errors.Is(err, service.ErrVisitRateLimited):
		return logAndBuildError(req, http.StatusTooManyRequests, "RATE_LIMITED", "too many events for this visitor", err)
	default:
		return logAndBuildError(req, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an internal server error occurred", err)
	}
}

func headerValue(headers map[string]string, name string) string {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}