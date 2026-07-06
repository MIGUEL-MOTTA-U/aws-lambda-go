package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/service"
)

// UploadService is the interface the controller depends on.
// Defined here following the project's pattern (see listing_controller.go).
type UploadService interface {
	UploadFiles(ctx context.Context, files []service.ParsedFile, req service.UploadFilesRequest, ownerID string) ([]model.Asset, error)
	GetAssetURL(ctx context.Context, id string, ownerID string) (model.Asset, error)
	ListEntityAssets(ctx context.Context, entityType model.AssetEntityType, entityID string) ([]model.Asset, error)
	DeleteAsset(ctx context.Context, id string, ownerID string) error
}

// UploadController handles HTTP routing and request parsing for all upload endpoints.
// It is intentionally thin: it parses the HTTP layer and delegates all logic
// to UploadService.
type UploadController struct {
	service UploadService
}

func NewUploadController(svc UploadService) *UploadController {
	return &UploadController{service: svc}
}

// HandleRequest routes incoming requests to the appropriate handler.
//
// Supported routes:
//
//	POST   /uploads                              — upload one or more files
//	GET    /uploads/{id}/url                    — get a URL for a private asset
//	DELETE /uploads/{id}                        — delete an asset
//	GET    /listings/{id}/media                 — list all media for a listing
func (c *UploadController) HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := req.RequestContext.HTTP.Method
	path := normalizePath(req.RawPath)

	switch {
	// POST /uploads
	case method == http.MethodPost && path == "/uploads":
		return c.handleUpload(ctx, req)

	// GET /uploads/{id}/url
	case method == http.MethodGet && isUploadURLPath(path):
		id := assetIDFromURLPath(path)
		return c.handleGetURL(ctx, req, id)

	// DELETE /uploads/{id}
	case method == http.MethodDelete && isUploadIDPath(path):
		id := assetIDFromPath(path)
		return c.handleDelete(ctx, req, id)

	// GET /listings/{id}/media
	case method == http.MethodGet && isListingMediaPath(path):
		listingID := listingIDFromMediaPath(path)
		return c.handleListMedia(ctx, req, listingID)

	case path == "/uploads" || strings.HasPrefix(path, "/uploads/"):
		return logAndBuildError(req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil), nil

	default:
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "route not found", nil), nil
	}
}

// handleUpload processes a multipart/form-data request with one or more files.
//
// Expected form fields:
//
//	entity_type  — one of: listing_photo, user_avatar, listing_pdf
//	entity_id    — ID of the owning listing or user
//	file         — one or more file parts (field name "file" or "files")
//
// The handler decodes the body (base64 if flagged by API Gateway), parses the
// multipart stream, and delegates concurrent upload to UploadService.
func (c *UploadController) handleUpload(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	ownerID := ownerIDFromRequest(req)
	if ownerID == "" {
		return logAndBuildError(req, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil), nil
	}

	files, fields, err := parseMultipartRequest(req)
	if err != nil {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "invalid multipart request: "+err.Error(), err), nil
	}
	if len(files) == 0 {
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", "at least one file is required", nil), nil
	}

	uploadReq := service.UploadFilesRequest{
		EntityType: model.AssetEntityType(strings.TrimSpace(fields["entity_type"])),
		EntityID:   strings.TrimSpace(fields["entity_id"]),
	}

	assets, err := c.service.UploadFiles(ctx, files, uploadReq, ownerID)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusCreated, assets)
}

// handleGetURL returns a resolved URL for a single asset.
// Private assets receive a short-lived presigned URL; public ones get the CDN URL.
func (c *UploadController) handleGetURL(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	ownerID := ownerIDFromRequest(req)
	if ownerID == "" {
		return logAndBuildError(req, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil), nil
	}

	asset, err := c.service.GetAssetURL(ctx, id, ownerID)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, asset)
}

// handleDelete soft-deletes an asset from the database and removes it from R2.
func (c *UploadController) handleDelete(ctx context.Context, req events.APIGatewayV2HTTPRequest, id string) (events.APIGatewayV2HTTPResponse, error) {
	ownerID := ownerIDFromRequest(req)
	if ownerID == "" {
		return logAndBuildError(req, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil), nil
	}

	if err := c.service.DeleteAsset(ctx, id, ownerID); err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusNoContent, nil)
}

// handleListMedia lists all confirmed assets for a listing with their resolved URLs.
func (c *UploadController) handleListMedia(ctx context.Context, req events.APIGatewayV2HTTPRequest, listingID string) (events.APIGatewayV2HTTPResponse, error) {
	assets, err := c.service.ListEntityAssets(ctx, model.EntityTypeListingPhoto, listingID)
	if err != nil {
		return c.errorToResponse(req, err), nil
	}

	return buildSuccessResponse(req, http.StatusOK, assets)
}

// ─── Error mapping ───────────────────────────────────────────────────────────

func (c *UploadController) errorToResponse(req events.APIGatewayV2HTTPRequest, err error) events.APIGatewayV2HTTPResponse {
	switch {
	case errors.Is(err, service.ErrInvalidUpload):
		return logAndBuildError(req, http.StatusBadRequest, "BAD_REQUEST", err.Error(), err)
	case errors.Is(err, service.ErrForbidden):
		return logAndBuildError(req, http.StatusForbidden, "FORBIDDEN", "you do not have permission to perform this action", err)
	case errors.Is(err, repository.ErrAssetNotFound):
		return logAndBuildError(req, http.StatusNotFound, "NOT_FOUND", "asset not found", err)
	case errors.Is(err, service.ErrUploadFailed):
		return logAndBuildError(req, http.StatusBadGateway, "UPLOAD_FAILED", "one or more files could not be uploaded", err)
	default:
		return logAndBuildError(req, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an internal server error occurred", err)
	}
}

// ─── Multipart parsing ───────────────────────────────────────────────────────

// parseMultipartRequest decodes and parses the multipart body from an API Gateway V2 event.
// API Gateway sets IsBase64Encoded = true when the body contains binary data.
func parseMultipartRequest(req events.APIGatewayV2HTTPRequest) ([]service.ParsedFile, map[string]string, error) {
	rawBody := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(rawBody)
		if err != nil {
			return nil, nil, errors.New("failed to decode base64 body")
		}
		rawBody = string(decoded)
	}

	contentType := req.Headers["content-type"]
	if contentType == "" {
		contentType = req.Headers["Content-Type"]
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return nil, nil, errors.New("content-type must be multipart/form-data")
	}

	boundary, ok := params["boundary"]
	if !ok || boundary == "" {
		return nil, nil, errors.New("multipart boundary is missing")
	}

	reader := multipart.NewReader(strings.NewReader(rawBody), boundary)

	var parsedFiles []service.ParsedFile
	fields := make(map[string]string)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errors.New("error reading multipart body")
		}

		data, err := io.ReadAll(part)
		part.Close()
		if err != nil {
			return nil, nil, errors.New("error reading part data")
		}

		filename := part.FileName()
		if filename != "" {
			// This part is a file
			ext := strings.ToLower(filepath.Ext(filename))
			ct := part.Header.Get("Content-Type")
			if ct == "" {
				ct = http.DetectContentType(data)
			}
			parsedFiles = append(parsedFiles, service.ParsedFile{
				Filename:    filename,
				Extension:   ext,
				ContentType: ct,
				Data:        data,
			})
		} else {
			// This part is a form field
			fields[part.FormName()] = string(data)
		}
	}

	return parsedFiles, fields, nil
}

// ─── Path helpers ────────────────────────────────────────────────────────────

// isUploadIDPath matches /uploads/{id} (exactly two segments)
func isUploadIDPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "uploads" && parts[1] != ""
}

// isUploadURLPath matches /uploads/{id}/url
func isUploadURLPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 3 && parts[0] == "uploads" && parts[1] != "" && parts[2] == "url"
}

// isListingMediaPath matches /listings/{id}/media
func isListingMediaPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 3 && parts[0] == "listings" && parts[1] != "" && parts[2] == "media"
}

func assetIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func assetIDFromURLPath(path string) string {
	return assetIDFromPath(path) // /uploads/{id}/url → parts[1]
}

func listingIDFromMediaPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// ownerIDFromRequest extracts the authenticated user's ID from the request.
// This assumes JWT claims are forwarded by API Gateway V2 as a context authorizer.
// Adapt this function to match your actual authentication mechanism.
func ownerIDFromRequest(req events.APIGatewayV2HTTPRequest) string {
	// Requests without an authorizer configured carry a nil Authorizer.
	if authorizer := req.RequestContext.Authorizer; authorizer != nil {
		// API Gateway V2 JWT authorizer injects claims into Authorizer.JWT.Claims
		if jwt := authorizer.JWT; jwt != nil {
			if sub, ok := jwt.Claims["sub"]; ok {
				return sub
			}
		}
		// Fallback: custom Lambda authorizer may inject into context
		if ctx := authorizer.Lambda; ctx != nil {
			if sub, ok := ctx["sub"]; ok {
				if s, ok := sub.(string); ok {
					return s
				}
			}
		}
	}
	// Stage 1 (pre-Cognito) escape hatch: uploads without an authorizer are
	// attributed to a fixed owner. Remove this variable once the JWT
	// authorizer is enabled in Stage 2 (see ai-notes.md).
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_UNAUTHENTICATED_UPLOADS")), "true") {
		return "stage1-anonymous"
	}
	return ""
}
