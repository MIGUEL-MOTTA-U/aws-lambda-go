package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/service"
)

type UserService interface {
	ListUsers(ctx context.Context) ([]model.User, error)
	GetUser(ctx context.Context, id string) (model.User, error)
	CreateUser(ctx context.Context, user model.User) (model.User, error)
	UpdateUser(ctx context.Context, id string, user model.User) (model.User, error)
	DeleteUser(ctx context.Context, id string) error
}

type UserController struct {
	service UserService
}

func NewUserController(service UserService) *UserController {
	return &UserController{service: service}
}

func (c UserController) HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := req.RequestContext.HTTP.Method
	path := normalizePath(req.RawPath)
	id := req.PathParameters["id"]

	switch {
	case method == http.MethodGet && path == "/users":
		return c.listUsers(ctx, req)
	case method == http.MethodGet && isUserIDPath(path):
		return c.getUser(ctx, req, idFromRequest(path, id))
	case method == http.MethodPost && path == "/users":
		return c.createUser(ctx, req)
	case method == http.MethodPut && isUserIDPath(path):
		return c.updateUser(ctx, req, idFromRequest(path, id))
	case method == http.MethodDelete && isUserIDPath(path):
		return c.deleteUser(ctx, req, idFromRequest(path, id))
	case path == "/users" || strings.HasPrefix(path, "/users/"):
		return logAndBuildError(req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil), nil
	default:
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "route not found", nil), nil
	}
}

func (c UserController) listUsers(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	users, err := c.service.ListUsers(ctx)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, users)
}

func (c UserController) getUser(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	user, err := c.service.GetUser(ctx, id)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, user)
}

func (c UserController) createUser(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	user, err := decodeUser(req.Body)
	if err != nil {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body", err), nil
	}

	createdUser, err := c.service.CreateUser(ctx, user)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusCreated, createdUser)
}

func (c UserController) updateUser(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	user, err := decodeUser(req.Body)
	if err != nil {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body", err), nil
	}

	updatedUser, err := c.service.UpdateUser(ctx, id, user)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, updatedUser)
}

func (c UserController) deleteUser(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	if err := c.service.DeleteUser(ctx, id); err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusNoContent, nil)
}

func decodeUser(body string) (model.User, error) {
	if strings.TrimSpace(body) == "" {
		return model.User{}, errors.New("request body is required")
	}

	var user model.User
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&user); err != nil {
		return model.User{}, fmt.Errorf("invalid json body: %w", err)
	}

	return user, nil
}

func (c UserController) errorToResponse(req events.APIGatewayV2HTTPRequest, err error) events.APIGatewayV2HTTPResponse {
	switch {
	case errors.Is(err, service.ErrInvalidUser):
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
	case errors.Is(err, repository.ErrUserNotFound):
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "user not found", err)
	case errors.Is(err, repository.ErrUserAlreadyExists):
		return logAndBuildError(req, http.StatusConflict, "CONFLICT", "user already exists", err)
	default:
		return logAndBuildError(req, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an internal server error occurred", err)
	}
}

func isUserIDPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "users" && parts[1] != ""
}
