package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"avito-kitchen/internal/domain"
)

type ErrorResponse struct {
	Code string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("[HTTP ERROR] encode json response: %v", err)
		}
	}
}

func writeError(w http.ResponseWriter, err error) {
	var status = http.StatusInternalServerError
	var code = "INTERNAL_SERVER_ERROR"
	var msg = "Внутренняя ошибка сервера"

	switch {
	case errors.Is(err, domain.ErrRestaurantNotFound):
		status = http.StatusNotFound
		code = "RESTAURANT_NOT_FOUND"
		msg = err.Error()

	case errors.Is(err, domain.ErrOrderNotFound):
		status = http.StatusNotFound
		code = "ORDER_NOT_FOUND"
		msg = err.Error()

	case errors.Is(err, domain.ErrMenuItemNotFound):
		status = http.StatusNotFound
		code = "MENU_ITEM_NOT_FOUND"
		msg = err.Error()

	case errors.Is(err, domain.ErrMenuItemUnavailable):
		status = http.StatusConflict
		code = "ITEM_UNAVAILABLE"
		msg = err.Error()

	case errors.Is(err, domain.ErrInvalidOrderStatus):
		status = http.StatusBadRequest
		code = "INVALID_ORDER_STATUS"
		msg = err.Error()
	}

	writeJSON(w, status, ErrorResponse{
		Code: code,
		Message: msg,
	})
}
