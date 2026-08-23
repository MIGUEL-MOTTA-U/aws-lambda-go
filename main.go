package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"rs-lambda-go/internal/auth"
	"rs-lambda-go/internal/controller"
	"rs-lambda-go/internal/localserver"
	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/service"
	"rs-lambda-go/internal/storage"
)

const (
	databaseConnectionEnv = "DATABASE_CONNECTION"

	// R2 environment variables: all required at startup.
	// R2_ENDPOINT      : https://<account-id>.r2.cloudflarestorage.com
	// R2_BUCKET_NAME   : name of the R2 bucket
	// R2_ACCESS_KEY_ID : R2 API token key ID (from Cloudflare Dashboard)
	// R2_SECRET_ACCESS_KEY: R2 API token secret
	// R2_PUBLIC_URL    : Cloudflare CDN custom domain for public assets
	r2EndpointEnv   = "R2_ENDPOINT"
	r2BucketNameEnv = "R2_BUCKET_NAME"
	r2AccessKeyEnv  = "R2_ACCESS_KEY_ID"
	r2SecretKeyEnv  = "R2_SECRET_ACCESS_KEY"
	r2PublicURLEnv  = "R2_PUBLIC_URL"
)

// TokenVerifier validates a bearer token and returns its claims.
// Satisfied by *auth.Verifier; stubbed in tests.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (map[string]string, error)
}

type Router struct {
	healthController    *controller.HealthController
	userController      *controller.UserController
	listingController   *controller.ListingController
	uploadController    *controller.UploadController
	visitController     *controller.VisitController
	analyticsController *controller.AnalyticsController
	// tokenVerifier protege las rutas de mutación. nil = guard deshabilitado
	// (desarrollo local sin COGNITO_ISSUER/COGNITO_AUDIENCE).
	tokenVerifier TokenVerifier
}

func (r Router) Route(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.HTTP.Method == http.MethodOptions {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusNoContent,
			Headers: map[string]string{
				"Access-Control-Allow-Origin":  allowedOrigin(req.Headers),
				"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
				"Access-Control-Allow-Headers": "content-type,authorization",
			},
		}, nil
	}

	if resp, authorized := r.authorize(ctx, &req); !authorized {
		return resp, nil
	}

	path := strings.TrimRight(req.RawPath, "/")
	if path == "/health" || path == "/ping" {
		return r.healthController.HandleRequest(ctx, req)
	}
	if path == "/users" || strings.HasPrefix(path, "/users/") {
		return r.userController.HandleRequest(ctx, req)
	}
	if path == "/listings" || strings.HasPrefix(path, "/listings/") {
		// /listings/{id}/media is handled by the upload controller
		if isListingsMediaRoute(path) {
			return r.uploadController.HandleRequest(ctx, req)
		}
		return r.listingController.HandleRequest(ctx, req)
	}
	if path == "/uploads" || strings.HasPrefix(path, "/uploads/") {
		return r.uploadController.HandleRequest(ctx, req)
	}
	if path == "/visits" || strings.HasPrefix(path, "/visits/") {
		return r.visitController.HandleRequest(ctx, req)
	}
	if path == "/analytics" || strings.HasPrefix(path, "/analytics/") {
		return r.analyticsController.HandleRequest(ctx, req)
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: 404,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"message":"route not found"}`,
	}, nil
}

// isListingsMediaRoute returns true for /listings/{id}/media paths,
// which are served by the upload controller instead of the listing controller.
func isListingsMediaRoute(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 3 && parts[0] == "listings" && parts[2] == "media"
}

// authorize enforces a valid Cognito JWT on mutation routes. Reads stay open
// (the public site consumes them without a session). On success the claims
// are injected into the request context with the same shape the API Gateway
// JWT authorizer uses, so ownerIDFromRequest keeps working unchanged.
func (r Router) authorize(ctx context.Context, req *events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, bool) {
	if r.tokenVerifier == nil || !requiresAuth(*req) {
		return events.APIGatewayV2HTTPResponse{}, true
	}

	token := bearerToken(req.Headers)
	if token == "" {
		return unauthorizedResponse("authentication required"), false
	}
	claims, err := r.tokenVerifier.Verify(ctx, token)
	if err != nil {
		log.Printf("[WARN] rejected token on %s %s: %v", req.RequestContext.HTTP.Method, req.RawPath, err)
		return unauthorizedResponse("invalid or expired token"), false
	}

	if req.RequestContext.Authorizer == nil {
		req.RequestContext.Authorizer = &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			JWT: &events.APIGatewayV2HTTPRequestContextAuthorizerJWTDescription{Claims: claims},
		}
	}
	return events.APIGatewayV2HTTPResponse{}, true
}

// requiresAuth marks every non-read request to the API resources as guarded.
// Exception: GET /analytics/* is guarded too — analytics data is for the
// admin panel only and must not leak via the public read endpoints.
func requiresAuth(req events.APIGatewayV2HTTPRequest) bool {
	method := req.RequestContext.HTTP.Method
	path := strings.TrimRight(req.RawPath, "/")

	// Analytics reads are guarded even though GETs elsewhere aren't.
	if method == http.MethodGet && (path == "/analytics" || strings.HasPrefix(path, "/analytics/")) {
		return true
	}
	if method == http.MethodGet || method == http.MethodOptions || method == http.MethodHead {
		return false
	}
	for _, prefix := range []string{"/listings", "/users", "/uploads"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func bearerToken(headers map[string]string) string {
	for key, value := range headers {
		if strings.EqualFold(key, "authorization") {
			if token, found := strings.CutPrefix(strings.TrimSpace(value), "Bearer "); found {
				return strings.TrimSpace(token)
			}
		}
	}
	return ""
}

func allowedOrigin(headers map[string]string) string {
	for key, value := range headers {
		if strings.EqualFold(key, "origin") {
			origin := strings.TrimSpace(value)
			switch origin {
			case "https://aura-urrea.vercel.app", "http://localhost:5173":
				return origin
			}
		}
	}
	return "https://aura-urrea.vercel.app"
}

func unauthorizedResponse(message string) events.APIGatewayV2HTTPResponse {
	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusUnauthorized,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"code":"UNAUTHORIZED","message":"%s","status":401}`, message),
	}
}

func main() {
	// When not running inside the AWS Lambda runtime, act as a plain HTTP
	// server for local development and load configuration from .env.
	runningLocally := os.Getenv("AWS_LAMBDA_RUNTIME_API") == ""
	if runningLocally {
		if err := localserver.LoadDotEnv(".env"); err != nil {
			panic(fmt.Sprintf("unable to load .env file: %v", err))
		}
	}

	// Database.
	databaseConnection := strings.TrimSpace(os.Getenv(databaseConnectionEnv))
	if databaseConnection == "" {
		panic(fmt.Sprintf("missing required environment variable %s", databaseConnectionEnv))
	}

	db, err := gorm.Open(postgres.Open(databaseConnection), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("unable to connect to database: %v", err))
	}

	// Connection pool tuning for AWS Lambda & Serverless Postgres (Neon / Supabase).
	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Sprintf("unable to get underlying sql.DB: %v", err))
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)

	// Auto-migrate tables for postgres
	err = db.AutoMigrate(&model.User{}, &model.Listing{}, &model.Asset{}, &model.Visit{})
	if err != nil {
		panic(fmt.Sprintf("unable to auto-migrate database schema: %v", err))
	}

	// Cloudflare R2 storage.
	r2Client, err := storage.NewR2Client(storage.R2Config{
		Endpoint:        mustEnv(r2EndpointEnv),
		BucketName:      mustEnv(r2BucketNameEnv),
		AccessKeyID:     os.Getenv(r2AccessKeyEnv),
		SecretAccessKey: os.Getenv(r2SecretKeyEnv),
		PublicURL:       mustEnv(r2PublicURLEnv),
	})
	if err != nil {
		panic(fmt.Sprintf("unable to create R2 storage client: %v", err))
	}

	// Dependency wiring.
	userRepo := repository.NewGormUserRepository(db)
	userService := service.NewUserService(userRepo)
	userController := controller.NewUserController(userService)

	listingRepo := repository.NewGormListingRepository(db)
	listingService := service.NewListingService(listingRepo)
	listingController := controller.NewListingController(listingService)

	assetRepo := repository.NewGormAssetRepository(db)
	uploadService := service.NewUploadService(assetRepo, r2Client, service.NewID)
	uploadController := controller.NewUploadController(uploadService)

	visitRepo := repository.NewGormVisitRepository(db)
	visitService := service.NewVisitService(visitRepo, service.NewID)
	visitController := controller.NewVisitController(visitService)

	analyticsService := service.NewAnalyticsService(visitRepo)
	analyticsController := controller.NewAnalyticsController(analyticsService)

	// Guard de autenticación para rutas de mutación.
	// Requiere COGNITO_ISSUER y COGNITO_AUDIENCE en producción (AWS Lambda).
	var tokenVerifier TokenVerifier
	if verifier := auth.NewVerifierFromEnv(); verifier != nil {
		tokenVerifier = verifier
		log.Printf("[INFO] Cognito JWT guard enabled for mutation routes")
	} else if !runningLocally {
		panic("COGNITO_ISSUER and COGNITO_AUDIENCE are strictly required in production (AWS Lambda environment)")
	} else {
		log.Printf("[WARN] Cognito JWT guard DISABLED (local development mode without COGNITO_ISSUER/COGNITO_AUDIENCE)")
	}

	healthController := controller.NewHealthController(db)

	router := Router{
		healthController:    healthController,
		userController:      userController,
		listingController:   listingController,
		uploadController:    uploadController,
		visitController:     visitController,
		analyticsController: analyticsController,
		tokenVerifier:       tokenVerifier,
	}

	if runningLocally {
		if err := localserver.Serve(router.Route); err != nil {
			panic(fmt.Sprintf("local server stopped: %v", err))
		}
		return
	}

	lambda.Start(router.Route)
}

// mustEnv reads a required environment variable and panics with a clear
// message if it is missing, consistent with databaseConnectionEnv handling.
func mustEnv(key string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		panic(fmt.Sprintf("missing required environment variable %s", key))
	}
	return val
}
