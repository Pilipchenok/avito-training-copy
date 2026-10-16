package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"avito-kitchen/internal/domain"
	"avito-kitchen/internal/handler"
)

type mockClientService struct {
	getRestaurantsFunc func(ctx context.Context) ([]domain.Restaurant, error)
	getMenuFunc        func(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error)
	createOrderFunc    func(ctx context.Context, restID uuid.UUID, address string, items []domain.OrderItem) (*domain.Order, error)
	getOrderFunc       func(ctx context.Context, orderID uuid.UUID) (*domain.Order, error)
}

func (m *mockClientService) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	if m.getRestaurantsFunc != nil {
		return m.getRestaurantsFunc(ctx)
	}
	return nil, nil
}

func (m *mockClientService) GetMenu(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error) {
	if m.getMenuFunc != nil {
		return m.getMenuFunc(ctx, restID)
	}
	return nil, nil
}

func (m *mockClientService) CreateOrder(ctx context.Context, restID uuid.UUID, address string, items []domain.OrderItem) (*domain.Order, error) {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, restID, address, items)
	}
	return nil, nil
}

func (m *mockClientService) GetOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	if m.getOrderFunc != nil {
		return m.getOrderFunc(ctx, orderID)
	}
	return nil, nil
}

type mockPartnerService struct {
	updateAvailabilityFunc func(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error
	updateOrderStatusFunc  func(ctx context.Context, orderID uuid.UUID, newStatus domain.OrderStatus) error
}

func (m *mockPartnerService) UpdateAvailability(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error {
	if m.updateAvailabilityFunc != nil {
		return m.updateAvailabilityFunc(ctx, restID, itemID, isAvailable)
	}
	return nil
}

func (m *mockPartnerService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, newStatus domain.OrderStatus) error {
	if m.updateOrderStatusFunc != nil {
		return m.updateOrderStatusFunc(ctx, orderID, newStatus)
	}
	return nil
}

func setupTestRouter(clientSvc handler.ClientService, partnerSvc handler.PartnerService) *http.ServeMux {
	mux := http.NewServeMux()

	if clientSvc != nil {
		clientH := handler.NewClientHandler(clientSvc)
		mux.HandleFunc("GET /api/v1/client/restaurants", clientH.GetRestaurants)
		mux.HandleFunc("GET /api/v1/client/restaurants/{restaurant_id}/menu", clientH.GetMenu)
		mux.HandleFunc("POST /api/v1/client/orders", clientH.CreateOrder)
		mux.HandleFunc("GET /api/v1/client/orders/{order_id}", clientH.GetOrder)
	}

	if partnerSvc != nil {
		partnerH := handler.NewPartnerHandler(partnerSvc)
		mux.HandleFunc("PUT /api/v1/partner/restaurants/{restaurant_id}/menu/{item_id}/availability", partnerH.UpdateAvailability)
		mux.HandleFunc("PATCH /api/v1/partner/orders/{order_id}/status", partnerH.UpdateStatus)
	}

	return mux
}

func TestClientHandler(t *testing.T) {
	t.Run("GET /api/v1/client/restaurants - Успех", func(t *testing.T) {
		mockSvc := &mockClientService{
			getRestaurantsFunc: func(ctx context.Context) ([]domain.Restaurant, error) {
				return []domain.Restaurant{
					{ID: uuid.New(), Name: "Додо", WebhookURL: "http://mock", IsActive: true},
				}, nil
			},
		}

		router := setupTestRouter(mockSvc, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/client/restaurants", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("ожидался статус 200 OK, получен %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("ожидался Content-Type application/json, получен %s", ct)
		}
	})

	t.Run("GET /api/v1/client/restaurants/{restaurant_id}/menu - Невалидный UUID", func(t *testing.T) {
		router := setupTestRouter(&mockClientService{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/client/restaurants/invalid-uuid-format/menu", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался статус 400 Bad Request, получен %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/client/orders - Ошибка валидации (пустое тело)", func(t *testing.T) {
		router := setupTestRouter(&mockClientService{}, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/client/orders", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался статус 400 Bad Request, получен %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/client/orders - Ошибка валидации (отрицательное количество)", func(t *testing.T) {
		payload := map[string]any{
			"restaurant_id":    uuid.New().String(),
			"delivery_address": "ул. Мира, 1",
			"items": []map[string]any{
				{"menu_item_id": uuid.New().String(), "quantity": -5},
			},
		}
		body, _ := json.Marshal(payload)

		router := setupTestRouter(&mockClientService{}, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/client/orders", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался статус 400 Bad Request, получен %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/client/orders - 409 Conflict при недоступном блюде", func(t *testing.T) {
		mockSvc := &mockClientService{
			createOrderFunc: func(ctx context.Context, restID uuid.UUID, address string, items []domain.OrderItem) (*domain.Order, error) {
				return nil, domain.ErrMenuItemUnavailable
			},
		}

		payload := map[string]any{
			"restaurant_id":    uuid.New().String(),
			"delivery_address": "ул. Мира, 1",
			"items": []map[string]any{
				{"menu_item_id": uuid.New().String(), "quantity": 1},
			},
		}
		body, _ := json.Marshal(payload)

		router := setupTestRouter(mockSvc, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/client/orders", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("ожидался статус 409 Conflict, получен %d", rec.Code)
		}
	})

	t.Run("POST /api/v1/client/orders - Успешное создание заказа (201 Created)", func(t *testing.T) {
		createdOrder := &domain.Order{
			ID:              uuid.New(),
			RestaurantID:    uuid.New(),
			Status:          domain.StatusCreated,
			TotalPrice:      1200,
			DeliveryAddress: "ул. Ленина, 5",
		}

		mockSvc := &mockClientService{
			createOrderFunc: func(ctx context.Context, restID uuid.UUID, address string, items []domain.OrderItem) (*domain.Order, error) {
				return createdOrder, nil
			},
		}

		payload := map[string]any{
			"restaurant_id":    createdOrder.RestaurantID.String(),
			"delivery_address": createdOrder.DeliveryAddress,
			"items": []map[string]any{
				{"menu_item_id": uuid.New().String(), "quantity": 2},
			},
		}
		body, _ := json.Marshal(payload)

		router := setupTestRouter(mockSvc, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/client/orders", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("ожидался статус 201 Created, получен %d", rec.Code)
		}
	})

	t.Run("GET /api/v1/client/orders/{order_id} - 404 Not Found", func(t *testing.T) {
		mockSvc := &mockClientService{
			getOrderFunc: func(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
				return nil, domain.ErrOrderNotFound
			},
		}

		router := setupTestRouter(mockSvc, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/client/orders/"+uuid.New().String(), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("ожидался статус 404 Not Found, получен %d", rec.Code)
		}
	})
}

func TestPartnerHandler(t *testing.T) {
	t.Run("PATCH /api/v1/partner/orders/{order_id}/status - 400 при недопустимом переходе статуса", func(t *testing.T) {
		mockSvc := &mockPartnerService{
			updateOrderStatusFunc: func(ctx context.Context, orderID uuid.UUID, newStatus domain.OrderStatus) error {
				return domain.ErrInvalidOrderStatus
			},
		}

		payload := map[string]string{"status": "READY"}
		body, _ := json.Marshal(payload)

		router := setupTestRouter(nil, mockSvc)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/"+uuid.New().String()+"/status", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("ожидался статус 400 Bad Request, получен %d", rec.Code)
		}
	})

	t.Run("PUT /api/v1/partner/restaurants/{restaurant_id}/menu/{item_id}/availability - Успех", func(t *testing.T) {
		mockSvc := &mockPartnerService{
			updateAvailabilityFunc: func(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error {
				return nil
			},
		}

		payload := map[string]bool{"is_available": false}
		body, _ := json.Marshal(payload)

		url := "/api/v1/partner/restaurants/" + uuid.New().String() + "/menu/" + uuid.New().String() + "/availability"
		router := setupTestRouter(nil, mockSvc)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("ожидался статус 200 OK, получен %d", rec.Code)
		}
	})

	t.Run("PUT - 500 при непредвиденной ошибке сервиса", func(t *testing.T) {
		mockSvc := &mockPartnerService{
			updateAvailabilityFunc: func(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error {
				return errors.New("database connection pool timeout")
			},
		}

		payload := map[string]bool{"is_available": true}
		body, _ := json.Marshal(payload)

		url := "/api/v1/partner/restaurants/" + uuid.New().String() + "/menu/" + uuid.New().String() + "/availability"
		router := setupTestRouter(nil, mockSvc)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("ожидался статус 500 Internal Server Error, получен %d", rec.Code)
		}
	})
}
