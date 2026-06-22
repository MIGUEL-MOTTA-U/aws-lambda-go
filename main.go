package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"database/sql"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	

	"rs-lambda-go/internal/controller"
	"rs-lambda-go/internal/repository"
	"rs-lambda-go/internal/service"
)

const (
	databaseConnectionEnv    = "DATABASE_CONNECTION"
)

var db *sql.DB

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

func init() {
	
}

func main() {
	databaseConnection := strings.TrimSpace(os.Getenv(databaseConnectionEnv))
	if databaseConnection == "" {
		panic(fmt.Sprintf("missing required environment variable %s", databaseConnectionEnv))
	}

	//cfg, err := config.LoadDefaultConfig(context.Background())
	// if err != nil {
	// 	panic(fmt.Sprintf("unable to load AWS config: %v", err))
	// }

	db, err := sql.Open("postgres", databaseConnection) // dynamodb.NewFromConfig(cfg)
	if err != nil {
		panic(fmt.Sprintln("Unable to connect to database"))
	}
	userRepo := repository.NewDynamoUserRepository(db, usersTable)
	userService := service.NewUserService(userRepo)
	userController := controller.NewUserController(userService)

	listingRepo := repository.NewDynamoListingRepository(db, listingsTable)
	listingService := service.NewListingService(listingRepo)
	listingController := controller.NewListingController(listingService)

	router := Router{
		userController:    userController,
		listingController: listingController,
	}

	lambda.Start(router.Route)
}
