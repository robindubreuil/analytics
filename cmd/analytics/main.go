// Analytics backend service - privacy-focused web analytics.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/robindubreuil/analytics/internal/api"
	"github.com/robindubreuil/analytics/internal/config"
	"github.com/robindubreuil/analytics/internal/db"
	"github.com/robindubreuil/analytics/internal/ingest"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("analytics %s (built %s)\n", version, buildTime)
		return
	}

	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	database.SetMaxOpenConns(cfg.MaxOpenConns)
	database.SetMaxIdleConns(cfg.MaxOpenConns)
	database.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	database.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ingestHandler := ingest.New(database, 3)
	dashboardHandler := api.New(database, cfg.DashboardAPIKey)

	rateLimiter, rateLimiterCleanup := api.RateLimiter(60, time.Minute)

	mux := http.NewServeMux()

	mux.Handle("/api/analytics", api.APIKey(cfg.APIKey)(
		rateLimiter(
			ingestHandler,
		),
	))

	dashboardHandler.RegisterRoutes(mux)

	var handler http.Handler = mux
	handler = api.Recovery(handler)
	handler = api.Logger(handler)

	allowedOrigins := []string{"*"}
	if cfg.CORSOrigins != "" {
		allowedOrigins = strings.Split(cfg.CORSOrigins, ",")
	}
	handler = api.CORS(allowedOrigins)(handler)

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.RetentionDays > 0 {
		go retentionJob(ctx, database, cfg.RetentionDays)
	}

	go shutdownOnSignal(server, cancel, rateLimiterCleanup)

	log.Printf("[Server] Starting on %s", cfg.Addr)
	log.Printf("[Server] Ingest endpoint: POST /api/analytics")
	log.Printf("[Server] Dashboard endpoints: /api/dashboard/*")
	if cfg.APIKey != "" {
		log.Printf("[Server] API key required for ingest")
	}
	if cfg.DashboardAPIKey != "" {
		log.Printf("[Server] API key required for dashboard")
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("[Server] Shutdown complete")
}

func shutdownOnSignal(server *http.Server, cancel context.CancelFunc, cleanupFuncs ...func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("[Server] Received signal: %v", sig)

	cancel()

	for _, fn := range cleanupFuncs {
		if fn != nil {
			fn()
		}
	}

	ctx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[Server] Shutdown error: %v", err)
	}
}

func retentionJob(ctx context.Context, database *sql.DB, retentionDays int) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	log.Printf("[Retention] Configured to retain events for %d days", retentionDays)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := db.DeleteOldEvents(database, retentionDays)
			if err != nil {
				log.Printf("[Retention] Error deleting old events: %v", err)
				continue
			}
			if deleted > 1000 {
				log.Printf("[Retention] Deleted %d old events, running VACUUM", deleted)
				if err := db.Vacuum(database); err != nil {
					log.Printf("[Retention] VACUUM error: %v", err)
				}
			} else if deleted > 0 {
				log.Printf("[Retention] Deleted %d old events", deleted)
			}
		}
	}
}
