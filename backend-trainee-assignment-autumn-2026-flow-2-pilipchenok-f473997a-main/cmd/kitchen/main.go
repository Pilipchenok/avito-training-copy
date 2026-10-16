package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"avito-kitchen/internal/handler"
	"avito-kitchen/internal/repository"
	"avito-kitchen/internal/service"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:pass@localhost:5432/kitchen_db?sslmode=disable"
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse db config: %v", err)
	}

	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("[INFO] connected to database successfully")

	repo := repository.NewRepository(pool)
	svc := service.NewService(repo)
	clientHandler := handler.NewClientHandler(svc)
	partnerHandler := handler.NewPartnerHandler(svc)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/client/restaurants", clientHandler.GetRestaurants)
	mux.HandleFunc("GET /api/v1/client/restaurants/{restaurant_id}/menu", clientHandler.GetMenu)
	mux.HandleFunc("POST /api/v1/client/orders", clientHandler.CreateOrder)
	mux.HandleFunc("GET /api/v1/client/orders/{order_id}", clientHandler.GetOrder)

	mux.HandleFunc("PUT /api/v1/partner/restaurants/{restaurant_id}/menu/{item_id}/availability", partnerHandler.UpdateAvailability)
	mux.HandleFunc("PATCH /api/v1/partner/orders/{order_id}/status", partnerHandler.UpdateStatus)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrappedMux := recoveryMiddleware(loggingMiddleware(mux))

	httpServer := &http.Server{
		Addr: ":" + serverPort,
		Handler: wrappedMux,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("[INFO] kitchen service starting on port :%s", serverPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] server forced to shutdown: %v", err)
	}

	log.Println("[INFO] server stopped")
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC RECOVER] %v", rec)
				http.Error(w, `{"code":"INTERNAL_SERVER_ERROR","message":"Непредвиденная ошибка"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s | duration: %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}
