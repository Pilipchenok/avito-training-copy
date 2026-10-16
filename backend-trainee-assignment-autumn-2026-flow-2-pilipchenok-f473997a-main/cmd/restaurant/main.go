package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type WebhookOrder struct {
	ID uuid.UUID `json:"id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	TotalPrice int `json:"total_price"`
	Items []any `json:"items"`
}

type UpdateStatusPayload struct {
	Status string `json:"status"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	kitchenURL := os.Getenv("KITCHEN_SERVICE_URL")
	if kitchenURL == "" {
		kitchenURL = "http://localhost:8080"
	}

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /webhook/orders", func(w http.ResponseWriter, r *http.Request) {
		var order WebhookOrder
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			log.Printf("[RESTAURANT ERROR] decode webhook: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		log.Printf("[RESTAURANT] Received new order %s for restaurant %s (total: %d RUB)",
			order.ID, order.RestaurantID, order.TotalPrice)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))

		go simulateCooking(order.ID, kitchenURL, httpClient)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr: ":" + port,
		Handler: mux,
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("[INFO] restaurant starting on port :%s (target kitchen: %s)", port, kitchenURL)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] shutting down restaurant")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] forced shutdown: %v", err)
	}

	log.Println("[INFO] restaurant stopped")
}

func simulateCooking(orderID uuid.UUID, kitchenURL string, client *http.Client) {
	time.Sleep(2 * time.Second)
	log.Printf("[RESTAURANT] Order %s: starting cooking", orderID)
	if err := sendStatusUpdate(kitchenURL, orderID, "COOKING", client); err != nil {
		log.Printf("[RESTAURANT ERROR] failed to set COOKING for order %s: %v", orderID, err)
		return
	}

	time.Sleep(3 * time.Second)
	log.Printf("[RESTAURANT] Order %s: cooking finished, dish is READY", orderID)
	if err := sendStatusUpdate(kitchenURL, orderID, "READY", client); err != nil {
		log.Printf("[RESTAURANT ERROR] failed to set READY for order %s: %v", orderID, err)
		return
	}
}

func sendStatusUpdate(baseURL string, orderID uuid.UUID, status string, client *http.Client) error {
	url := fmt.Sprintf("%s/api/v1/partner/orders/%s/status", baseURL, orderID)

	body, err := json.Marshal(UpdateStatusPayload{Status: status})
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create patch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
