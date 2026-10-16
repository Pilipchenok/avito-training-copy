package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"avito-kitchen/internal/domain"
)

type PartnerService interface {
	UpdateAvailability(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, newStatus domain.OrderStatus) error
}

type PartnerHandler struct {
	service PartnerService
}

func NewPartnerHandler(s PartnerService) *PartnerHandler {
	return &PartnerHandler{service: s}
}

type updateAvailabilityReq struct {
	IsAvailable bool `json:"is_available"`
}

func (h *PartnerHandler) UpdateAvailability(w http.ResponseWriter, r *http.Request) {
	restID, err := uuid.Parse(r.PathValue("restaurant_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_UUID", Message: "Некорректный ID заведения"})
		return
	}

	itemID, err := uuid.Parse(r.PathValue("item_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_UUID", Message: "Некорректный ID позиции меню"})
		return
	}

	var req updateAvailabilityReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "BAD_REQUEST", Message: "Некорректный JSON"})
		return
	}

	if err := h.service.UpdateAvailability(r.Context(), restID, itemID, req.IsAvailable); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

type updateStatusReq struct {
	Status domain.OrderStatus `json:"status"`
}

func (h *PartnerHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(r.PathValue("order_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "INVALID_UUID", Message: "Некорректный ID заказа"})
		return
	}

	var req updateStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: "BAD_REQUEST", Message: "Некорректный JSON"})
		return
	}

	if err := h.service.UpdateOrderStatus(r.Context(), orderID, req.Status); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
