package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func defaultHeaders() map[string]string {
	return map[string]string{
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
		"Access-Control-Allow-Origin":  "*",
		"Content-Type":                 "application/json",
	}
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func idFromRequest(path string, pathParameterID string) string {
	if strings.TrimSpace(pathParameterID) != "" {
		return pathParameterID
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 {
		return parts[1]
	}

	return ""
}

func logAndBuildError(req events.APIGatewayV2HTTPRequest, statusCode int, code string, clientMsg string, internalErr error) events.APIGatewayV2HTTPResponse {
	requestID := req.RequestContext.RequestID
	method := req.RequestContext.HTTP.Method
	path := req.RawPath

	if internalErr != nil {
		log.Printf("[ERROR] RequestID: %s | %s %s | Code: %s | ClientMessage: %s | InternalError: %v",
			requestID, method, path, code, clientMsg, internalErr)
	} else {
		log.Printf("[WARN] RequestID: %s | %s %s | Code: %s | ClientMessage: %s",
			requestID, method, path, code, clientMsg)
	}

	payload := APIError{
		Code:    code,
		Message: clientMsg,
	}

	body, _ := json.Marshal(payload)

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    defaultHeaders(),
		Body:       string(body),
	}
}

func buildSuccessResponse(req events.APIGatewayV2HTTPRequest, statusCode int, payload any) (events.APIGatewayV2HTTPResponse, error) {
	requestID := req.RequestContext.RequestID
	method := req.RequestContext.HTTP.Method
	path := req.RawPath

	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return logAndBuildError(req, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an internal server error occurred", err), nil
		}
	}

	log.Printf("[INFO] RequestID: %s | %s %s | Status: %d", requestID, method, path, statusCode)

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    defaultHeaders(),
		Body:       string(body),
	}, nil
}
