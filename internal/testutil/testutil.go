package testutil

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/robindubreuil/analytics/internal/api"
	"github.com/robindubreuil/analytics/internal/db"
	"github.com/robindubreuil/analytics/internal/ingest"
)

type TestDB struct {
	DB   *sql.DB
	Path string
}

func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	unique := fmt.Sprintf("%d_%s", time.Now().UnixNano(), t.Name())
	path := "/tmp/analytics_test_" + unique + ".db"

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		os.Remove(path)
	})

	return &TestDB{DB: database, Path: path}
}

type TestServer struct {
	Server        *httptest.Server
	DB            *sql.DB
	DashboardKey  string
}

func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	tdb := NewTestDB(t)

	ingestHandler := ingest.New(tdb.DB, 3)
	dashboardKey := "test-dashboard-key"
	dashboardHandler := api.New(tdb.DB, dashboardKey)

	mux := http.NewServeMux()

	rateLimiter, _ := api.RateLimiter(100, time.Minute)
	mux.Handle("/api/analytics", rateLimiter(
		api.APIKey("test-api-key")(ingestHandler),
	))

	mux.HandleFunc("GET /api/test-panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic from integration test")
	})

	dashboardHandler.RegisterRoutes(mux)

	var handler http.Handler = mux
	handler = api.Recovery(handler)
	handler = api.Logger(handler)
	handler = api.CORS([]string{"*"})(handler)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &TestServer{
		Server:       server,
		DB:           tdb.DB,
		DashboardKey: dashboardKey,
	}
}
