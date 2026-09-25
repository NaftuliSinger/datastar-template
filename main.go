package main

import (
	"net/http"

	"context"
	"database/sql"
	_ "embed"
	"log"
	"strconv"

	_ "modernc.org/sqlite"

	"github.com/naftulisinger/datastar-template/internal/crypto"
	"github.com/naftulisinger/datastar-template/internal/db"
	"github.com/naftulisinger/datastar-template/internal/server"
)

// same schema file sqlc reads, baked into the binary and run on startup
//
//go:embed sql/schema.sql
var ddl string

func main() {
	ctx := context.Background()

	sqliteDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}

	// every new connection to ":memory:" gets its own empty database,
	// so keep the pool to a single connection that all requests share
	sqliteDB.SetMaxOpenConns(1)

	// create tables
	if _, err := sqliteDB.ExecContext(ctx, ddl); err != nil {
		log.Fatal(err)
	}

	queries := db.New(sqliteDB)

	// generate some random todos
	generateRandomTodos(queries, 5)

	// stream live crypto prices (btc, eth, xrp) in the background
	cryptoEngine := crypto.NewEngine()
	go cryptoEngine.Run(ctx)

	s := server.NewServer("localhost", "8080", http.NewServeMux(), queries, cryptoEngine)
	s.RegisterRoutes()
	log.Fatal(s.Start())
}

// sample data so the todo page isn't empty on first run
func generateRandomTodos(queries *db.Queries, count int) {
	for i := 0; i < count; i++ {
		description := "Todo " + strconv.Itoa(i+1)
		_, err := queries.CreateTodo(context.Background(), db.CreateTodoParams{
			Description: description,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}
