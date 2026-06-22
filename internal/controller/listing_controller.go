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

type ListingService interface {
	ListListings(ctx context.Context) ([]model.Listing, error)
	GetListing(ctx context.Context, id string) (model.Listing, error)
	CreateListing(ctx context.Context, listing model.Listing) (model.Listing, error)
	UpdateListing(ctx context.Context, id string, listing model.Listing) (model.Listing, error)
	DeleteListing(ctx context.Context, id string) error
}

type ListingController struct {
	service ListingService
}

func NewListingController(service ListingService) *ListingController {
	return &ListingController{service: service}
}

func (c ListingController) HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := req.RequestContext.HTTP.Method
	path := normalizePath(req.RawPath)
	id := req.PathParameters["id"]

	switch {
	case method == http.MethodGet && path == "/listings":
		return c.listListings(ctx, req)
	case method == http.MethodGet && isListingIDPath(path):
		return c.getListing(ctx, req, idFromRequest(path, id))
	case method == http.MethodPost && path == "/listings":
		return c.createListing(ctx, req)
	case method == http.MethodPut && isListingIDPath(path):
		return c.updateListing(ctx, req, idFromRequest(path, id))
	case method == http.MethodDelete && isListingIDPath(path):
		return c.deleteListing(ctx, req, idFromRequest(path, id))
	case path == "/listings" || strings.HasPrefix(path, "/listings/"):
		return logAndBuildError(req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil), nil
	default:
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "route not found", nil), nil
	}
}

func (c ListingController) listListings(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	listings, err := c.service.ListListings(ctx)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, listings)
}

func (c ListingController) getListing(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	listing, err := c.service.GetListing(ctx, id)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, listing)
}

func (c ListingController) createListing(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	listing, err := decodeListing(req.Body)
	if err != nil {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body", err), nil
	}

	createdListing, err := c.service.CreateListing(ctx, listing)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusCreated, createdListing)
}

func (c ListingController) updateListing(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	listing, err := decodeListing(req.Body)
	if err != nil {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body", err), nil
	}

	updatedListing, err := c.service.UpdateListing(ctx, id, listing)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, updatedListing)
}

func (c ListingController) deleteListing(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	if err := c.service.DeleteListing(ctx, id); err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusNoContent, nil)
}

func decodeListing(body string) (model.Listing, error) {
	if strings.TrimSpace(body) == "" {
		return model.Listing{}, errors.New("request body is required")
	}

	var listing model.Listing
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&listing); err != nil {
		return model.Listing{}, fmt.Errorf("invalid json body: %w", err)
	}

	return listing, nil
}

func (c ListingController) errorToResponse(req events.APIGatewayV2HTTPRequest, err error) events.APIGatewayV2HTTPResponse {
	switch {
	case errors.Is(err, service.ErrInvalidListing):
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
	case errors.Is(err, repository.ErrListingNotFound):
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "listing not found", err)
	case errors.Is(err, repository.ErrListingAlreadyExists):
		return logAndBuildError(req, http.StatusConflict, "CONFLICT", "listing already exists", err)
	default:
		return logAndBuildError(req, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an internal server error occurred", err)
	}
}

func isListingIDPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "listings" && parts[1] != ""
}
