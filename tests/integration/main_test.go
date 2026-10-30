//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mujtaba91/ticketing-api/internal/config"
	"github.com/mujtaba91/ticketing-api/internal/database"
	"github.com/mujtaba91/ticketing-api/internal/server"
)

// baseURL is the address of the API under test, shared by every test in the package.
var baseURL string

// testPool lets a test check the database directly, not only the API's answers.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	code, err := run(m)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(code)
}

// run exists so the deferred cleanup still happens before TestMain calls os.Exit.
func run(m *testing.M) (int, error) {
	ctx := context.Background()

	migrations, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil {
		return 0, fmt.Errorf("find migrations: %w", err)
	}
	if len(migrations) == 0 {
		return 0, errors.New("find migrations: no .up.sql files")
	}

	// Same image as docker-compose.yml; the up migrations run as init scripts, in file name order.
	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("ticketing"),
		postgres.WithUsername("ticketing"),
		postgres.WithPassword("ticketing"),
		postgres.WithOrderedInitScripts(migrations...),
		postgres.BasicWaitStrategies(),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(pg); err != nil {
			log.Printf("terminate container: %v", err)
		}
	}()
	if err != nil {
		return 0, fmt.Errorf("start postgres: %w", err)
	}

	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 0, fmt.Errorf("connection string: %w", err)
	}
	pool, err := database.NewPool(ctx, url)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	testPool = pool

	srv := httptest.NewServer(server.NewRouter(pool, config.Config{
		JWTSecret:       "integration-test-secret-at-least-32-chars",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: time.Hour,
	}))
	defer srv.Close()
	baseURL = srv.URL

	return m.Run(), nil
}
