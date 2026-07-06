package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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

type Router struct {
	userController    *controller.UserController
	listingController *controller.ListingController
	uploadController  *controller.UploadController
}

func (r Router) Route(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	path := strings.TrimRight(req.RawPath, "/")
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

	// Auto-migrate tables for postgres
	err = db.AutoMigrate(&model.User{}, &model.Listing{}, &model.Asset{})
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

	router := Router{
		userController:    userController,
		listingController: listingController,
		uploadController:  uploadController,
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
