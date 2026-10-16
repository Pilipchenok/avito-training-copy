package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"avito-kitchen/internal/domain"
)

type ClientService interface {
	GetRestaurants(ctx context.Context) ([]domain.Restaurant, error)
	GetMenu(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error)
	CreateOrder(ctx context.Context, restID uuid.UUID, address string, items []domain.OrderItem) (*domain.Order, error)
	GetOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error)
}

type ClientHandler struct {
	service ClientService
}

func NewClientHandler(s ClientService) *ClientHandler {
	return &ClientHandler{service: s}
}

func (h *ClientHandler) GetRestaurants(w http.ResponseWriter, r *http.Request) {
	rests, err := h.service.GetRestaurants(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rests)
}

func (h *ClientHandler) GetMenu(w http.ResponseWriter, r *http.Request) {
	restID, err := uuid.Parse(r.PathValue("restaurant_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_UUID", Message: "Некорректный ID ресторана"})
		return
	}

	menu, err := h.service.GetMenu(r.Context(), restID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, menu)
}

type createOrderItemReq struct {
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Quantity int `json:"quantity"`
}

type createOrderReq struct {
	RestaurantID uuid.UUID `json:"restaurant_id"`
	DeliveryAddress string `json:"delivery_address"`
	Items []createOrderItemReq `json:"items"`
}

func (h *ClientHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "BAD_REQUEST", Message: "Некорректное тело JSON"})
		return
	}

	if req.RestaurantID == uuid.Nil || req.DeliveryAddress == "" || len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "VALIDATION_FAILED", Message: "Заполните все обязательные поля"})
		return
	}

	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Quantity <= 0 {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_QUANTITY", Message: "Количество товара должно быть больше 0"})
			return
		}
		items = append(items, domain.OrderItem{
			MenuItemID: it.MenuItemID,
			Quantity:   it.Quantity,
		})
	}

	order, err := h.service.CreateOrder(r.Context(), req.RestaurantID, req.DeliveryAddress, items)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (h *ClientHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(r.PathValue("order_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_UUID", Message: "Некорректный ID заказа"})
		return
	}

	order, err := h.service.GetOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, order)
}
