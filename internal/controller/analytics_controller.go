package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"rs-lambda-go/internal/service"
)

// AnalyticsService is the subset of service.AnalyticsService used by
// the controller. Kept narrow to avoid leaking implementation into
// the controller package.
type AnalyticsService interface {
	TopListings(ctx context.Context, window string, limit int) ([]service.ListingVisitStats, error)
}

type AnalyticsController struct {
	service AnalyticsService
}

func NewAnalyticsController(svc AnalyticsService) *AnalyticsController {
	return &AnalyticsController{service: svc}
}

// HandleRequest routes GET /analytics/listings. Protected by the
// Cognito JWT guard registered in main.go (requiresAuth), so anonymous
// callers cannot read this endpoint.
func (c AnalyticsController) HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := req.RequestContext.HTTP.Method
	path := normalizePath(req.RawPath)

	if path != "/analytics/listings" {
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "route not found", nil), nil
	}

	switch method {
	case http.MethodGet:
		return c.topListings(ctx, req)
	default:
		return logAndBuildError(req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil), nil
	}
}

func (c AnalyticsController) topListings(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	window := queryParam(req.QueryStringParameters, "window")
	limit := queryParamInt(req.QueryStringParameters, "limit", 50)

	stats, err := c.service.TopListings(ctx, window, limit)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, map[string]any{
		"window": canonicalWindow(window),
		"items":  stats,
	})
}

func (c AnalyticsController) errorToResponse(req events.APIGatewayV2HTTPRequest, err error) events.APIGatewayV2HTTPResponse {
	switch {
	case errors.Is(err, service.ErrInvalidWindow):
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
	default:
		return logAndBuildError(req, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an internal server error occurred", err)
	}
}

func queryParam(params map[string]string, name string) string {
	if params == nil {
		return ""
	}
	return strings.TrimSpace(params[name])
}

// queryParamInt parses an integer query param with a default fallback.
// Non-numeric values fall back to the default silently — the response
// shape stays predictable even if a client sends garbage.
func queryParamInt(params map[string]string, name string, def int) int {
	raw := queryParam(params, name)
	if raw == "" {
		return def
	}
	var n int
	for _, c := range raw {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return def
	}
	return n
}

// canonicalWindow mirrors the service's default so the response shape
// stays honest when the caller omits ?window=.
func canonicalWindow(window string) string {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "30d":
		return "30d"
	default:
		return "7d"
	}
}