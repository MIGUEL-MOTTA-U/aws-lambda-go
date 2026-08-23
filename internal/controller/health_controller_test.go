package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestHealthController_MethodNotAllowed(t *testing.T) {
	ctrl := NewHealthController(nil)
	req := events.APIGatewayV2HTTPRequest{
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: http.MethodPost,
			},
		},
	}

	resp, err := ctrl.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestHealthController_NilDB(t *testing.T) {
	ctrl := NewHealthController(nil)
	req := events.APIGatewayV2HTTPRequest{
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: http.MethodGet,
			},
		},
	}

	resp, err := ctrl.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}

	var healthResp HealthResponse
	if err := json.Unmarshal([]byte(resp.Body), &healthResp); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if healthResp.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", healthResp.Status)
	}
	if healthResp.Database != "disconnected" {
		t.Errorf("expected database 'disconnected', got %q", healthResp.Database)
	}
	if healthResp.Timestamp == "" {
		t.Error("expected timestamp to be non-empty")
	}
}
