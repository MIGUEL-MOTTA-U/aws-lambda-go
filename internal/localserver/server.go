// Package localserver runs the Lambda handler as a plain HTTP server for
// local development. It adapts net/http requests into API Gateway V2 events,
// so the exact same routing/controller code path is exercised locally and in
// AWS. In production CORS is configured on the API Gateway; here the server
// replies to preflight requests itself.
package localserver

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// Handler is the signature of the Lambda entry point being adapted.
type Handler func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

const (
	defaultPort   = "8080"
	defaultOrigin = "*"
)

// Serve starts an HTTP server on PORT (default 8080) that forwards every
// request to the Lambda handler. It blocks until the server stops.
func Serve(handler Handler) error {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}

	origin := strings.TrimSpace(os.Getenv("ACCESS_CONTROL_ALLOW_ORIGIN"))
	if origin == "" {
		origin = defaultOrigin
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeCORSHeaders(w, origin)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		event, err := toAPIGatewayEvent(r)
		if err != nil {
			http.Error(w, `{"code":"BAD_REQUEST","message":"unable to read request body","status":400}`, http.StatusBadRequest)
			return
		}

		resp, err := handler(r.Context(), event)
		if err != nil {
			http.Error(w, `{"code":"INTERNAL_SERVER_ERROR","message":"handler error","status":500}`, http.StatusInternalServerError)
			return
		}

		for key, value := range resp.Headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(resp.StatusCode)

		body := resp.Body
		if resp.IsBase64Encoded {
			decoded, decodeErr := base64.StdEncoding.DecodeString(body)
			if decodeErr == nil {
				body = string(decoded)
			}
		}
		_, _ = io.WriteString(w, body)
	})

	log.Printf("[INFO] local API server listening on http://localhost:%s", port)
	return http.ListenAndServe(":"+port, mux)
}

func writeCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "content-type,authorization")
}

// toAPIGatewayEvent converts a net/http request into the API Gateway V2 event
// shape the Lambda handler expects. Binary payloads (multipart uploads) are
// base64-encoded exactly like API Gateway does.
func toAPIGatewayEvent(r *http.Request) (events.APIGatewayV2HTTPRequest, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return events.APIGatewayV2HTTPRequest{}, err
	}

	headers := make(map[string]string, len(r.Header))
	for key, values := range r.Header {
		// API Gateway V2 forwards header names lowercased.
		headers[strings.ToLower(key)] = strings.Join(values, ",")
	}

	contentType := r.Header.Get("Content-Type")
	isBinary := strings.HasPrefix(contentType, "multipart/") ||
		strings.HasPrefix(contentType, "application/octet-stream")

	body := string(bodyBytes)
	if isBinary {
		body = base64.StdEncoding.EncodeToString(bodyBytes)
	}

	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		queryParams[key] = strings.Join(values, ",")
	}

	return events.APIGatewayV2HTTPRequest{
		RawPath:               r.URL.Path,
		RawQueryString:        r.URL.RawQuery,
		Headers:               headers,
		QueryStringParameters: queryParams,
		Body:                  body,
		IsBase64Encoded:       isBinary,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			RequestID: "local-" + r.Method + r.URL.Path,
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: r.Method,
				Path:   r.URL.Path,
			},
		},
	}, nil
}
