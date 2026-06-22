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
	"rs-lambda-go/internal/model"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/service"
)

const (
	databaseConnectionEnv = "DATABASE_CONNECTION"
)

type Router struct {
	userController    *controller.UserController
	listingController *controller.ListingController
}

func (r Router) Route(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	path := strings.TrimRight(req.RawPath, "/")
	if path == "/users" || strings.HasPrefix(path, "/users/") {
		return r.userController.HandleRequest(ctx, req)
	}
	if path == "/listings" || strings.HasPrefix(path, "/listings/") {
		return r.listingController.HandleRequest(ctx, req)
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: 404,
		Headers: map[string]string{
			"Access-Control-Allow-Headers": "Content-Type",
			"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
			"Access-Control-Allow-Origin":  "*",
			"Content-Type":                 "application/json",
		},
		Body: `{"message":"route not found"}`,
	}, nil
}

func main() {
	databaseConnection := strings.TrimSpace(os.Getenv(databaseConnectionEnv))
	if databaseConnection == "" {
		panic(fmt.Sprintf("missing required environment variable %s", databaseConnectionEnv))
	}

	db, err := gorm.Open(postgres.Open(databaseConnection), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("unable to connect to database: %v", err))
	}

	// Auto-migrate tables for postgres
	err = db.AutoMigrate(&model.User{}, &model.Listing{})
	if err != nil {
		panic(fmt.Sprintf("unable to auto-migrate database schema: %v", err))
	}

	userRepo := repository.NewGormUserRepository(db)
	userService := service.NewUserService(userRepo)
	userController := controller.NewUserController(userService)

	listingRepo := repository.NewGormListingRepository(db)
	listingService := service.NewListingService(listingRepo)
	listingController := controller.NewListingController(listingService)

	router := Router{
		userController:    userController,
		listingController: listingController,
	}

	lambda.Start(router.Route)
}
