// Command server is the rejected.ai backend entrypoint. It loads
// config, connects to MongoDB, builds the configured LLM provider, ensures
// indexes, and serves the HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dharmendra/rejected.ai/internal/api"
	"github.com/dharmendra/rejected.ai/internal/config"
	"github.com/dharmendra/rejected.ai/internal/llm"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[BOOT] config: %v", err)
	}
	log.Printf("[BOOT] config loaded: backend=%s model=%s db=%s", cfg.LLMBackend, cfg.OllamaModel, cfg.MongoDB)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, err := store.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("[BOOT] mongo: %v", err)
	}
	log.Printf("[BOOT] mongo connected: %s", cfg.MongoURI)

	if err := st.EnsureIndexes(ctx); err != nil {
		log.Fatalf("[BOOT] indexes: %v", err)
	}
	log.Printf("[BOOT] indexes ensured")

	// Clean up any orphaned report progress documents from previous server processes.
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, _ = st.Coll(store.CollReportProgress).UpdateMany(cleanupCtx,
		bson.D{{Key: "status", Value: "generating"}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "status", Value: "failed"},
			{Key: "error", Value: "Server restarted during generation"},
			{Key: "updated_at", Value: time.Now().UTC()},
		}}},
	)
	cleanupCancel()

	provider, err := llm.New(cfg)
	if err != nil {
		log.Fatalf("[BOOT] llm: %v", err)
	}

	srv := api.NewServer(cfg, st, provider)
	httpServer := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: srv.Routes(),
	}

	go func() {
		log.Printf("[BOOT] listening on %s", cfg.ServerAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[BOOT] http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Printf("[BOOT] shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = st.Disconnect(shutdownCtx)
}
