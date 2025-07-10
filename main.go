package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	"log"
	"net/http"
	"os"
	api "shoppinglist/api"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	apiHandler   http.Handler
	dbPool       *pgxpool.Pool
	migrationUrl = "file://migration/"
)

func runDbMigrations(migrationURL string, dbSource string) {
	migration, err := migrate.New(migrationURL, dbSource)
	if err != nil {
		log.Fatal("Cannot create new migrate instance")
	}

	if err = migration.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal("failed to run migrate up")
	}

	log.Printf("Db migrated successfully")
}

func init() {
	databaseUrl := os.Getenv("DB_URL")
	dbpool, err := pgxpool.New(context.Background(), databaseUrl)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	runDbMigrations(migrationUrl, databaseUrl)

	var repo api.ShoppingListEntityRepository = api.NewShoppingListRepository(dbpool)
	service := api.NewShoppingListService(&repo)
	strictHandler := api.NewStrictHandler(service, nil)
	mux := http.NewServeMux()

	apiHandler = api.HandlerFromMux(strictHandler, mux)
}

func main() {
	defer dbPool.Close()
	lambda.Start(httpadapter.New(apiHandler).ProxyWithContext)
}
