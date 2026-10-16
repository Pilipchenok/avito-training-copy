package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	StatusCreated OrderStatus = "CREATED"
	StatusCooking OrderStatus = "COOKING"
	StatusReady OrderStatus = "READY"
	StatusCancelled OrderStatus = "CANCELLED"
)

type Restaurant struct {
	ID uuid.UUID `json:"id"`
	Name string `json:"name"`
	WebhookURL string `json:"webhook_url"`
	IsActive bool `json:"is_active"`
}

type MenuItem struct {
	ID uuid.UUID `json:"id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Price int `json:"price"`
	IsAvailable bool `json:"is_available"`
}

type OrderItem struct {
	ID uuid.UUID `json:"id"`
	OrderID uuid.UUID `json:"order_id"`
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Name string `json:"name,omitempty"`
	Quantity int `json:"quantity"`
	PriceAtOrder int `json:"price"`
}

type Order struct {
	ID uuid.UUID `json:"id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	Status OrderStatus `json:"status"`
	TotalPrice int `json:"total_price"`
	DeliveryAddress string `json:"delivery_address"`
	Items []OrderItem `json:"items"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}