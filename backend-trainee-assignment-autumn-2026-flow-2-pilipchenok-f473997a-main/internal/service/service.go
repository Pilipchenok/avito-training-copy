package service

import (
	"context"
	"net/http"
	"time"
	"log"
	"encoding/json"
	"bytes"

	"github.com/google/uuid"

	"avito-kitchen/internal/domain"
)

type OrderRepository interface {
	GetRestaurants(ctx context.Context) ([]domain.Restaurant, error)
	GetRestaurantByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error)
	UpdateMenuItemAvailability(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error
	GetMenuByRestaurantID(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error)
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status domain.OrderStatus) error
	CreateOrder(ctx context.Context, order *domain.Order, itemRequests []domain.OrderItem) error
}

type Service struct {
	repo OrderRepository
	httpClient *http.Client
}

func NewService(repo OrderRepository) *Service {
	return &Service{repo: repo, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (s *Service) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	rests, err := s.repo.GetRestaurants(ctx)
	if err != nil {
		return nil, err
	}
	return rests, nil
}

func (s *Service) GetMenu(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error) {
	menu, err := s.repo.GetMenuByRestaurantID(ctx, restID)
	if err != nil {
		return nil, err
	}
	return menu, nil
}

func (s *Service) UpdateAvailability(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error {
	return s.repo.UpdateMenuItemAvailability(ctx, restID, itemID, isAvailable)
}

func (s *Service) GetOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (s *Service) CreateOrder(ctx context.Context, restID uuid.UUID, address string, itemRequests []domain.OrderItem) (*domain.Order, error) {
	order := &domain.Order{
		ID: uuid.New(), RestaurantID: restID,
		Status: domain.StatusCreated, DeliveryAddress: address,
	}

	err := s.repo.CreateOrder(ctx, order, itemRequests)
	if err != nil {
		return nil, err
	}
	go s.notifyRestaurant(order)
	return order, nil
}

func (s *Service) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, newStatus domain.OrderStatus) error {
	currentOrder, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	if !isValidTransition(currentOrder.Status, newStatus) {
		return domain.ErrInvalidOrderStatus
	}

	return s.repo.UpdateOrderStatus(ctx, orderID, newStatus)
}

func isValidTransition(from, to domain.OrderStatus) bool {
	if to == domain.StatusCancelled {
		return from != domain.StatusReady && from != domain.StatusCancelled
	}

	switch from {
	case domain.StatusCreated:
		return to == domain.StatusCooking
	case domain.StatusCooking:
		return to == domain.StatusReady
	default:
		return false
	}
}

func (s *Service) notifyRestaurant(order *domain.Order) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rest, err := s.repo.GetRestaurantByID(ctx, order.RestaurantID)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] failed to fetch restaurant %s: %v", order.RestaurantID, err)
		return
	}

	body, err := json.Marshal(order)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] failed to marshal order %s: %v", order.ID, err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rest.WebhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[WEBHOOK ERROR] failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[WEBHOOK ERROR] restaurant webhook failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[WEBHOOK WARN] restaurant returned fail status: %d", resp.StatusCode)
	}
}
