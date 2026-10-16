package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"avito-kitchen/internal/domain"
	"avito-kitchen/internal/service"
)

type mockRepo struct {
	getRestaurantsFunc             func(ctx context.Context) ([]domain.Restaurant, error)
	getRestaurantByIDFunc          func(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error)
	updateMenuItemAvailabilityFunc func(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error
	getMenuByRestaurantIDFunc      func(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error)
	getOrderByIDFunc               func(ctx context.Context, orderID uuid.UUID) (*domain.Order, error)
	updateOrderStatusFunc          func(ctx context.Context, orderID uuid.UUID, status domain.OrderStatus) error
	createOrderFunc                func(ctx context.Context, order *domain.Order, itemRequests []domain.OrderItem) error
}

func (m *mockRepo) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	if m.getRestaurantsFunc != nil {
		return m.getRestaurantsFunc(ctx)
	}
	return nil, nil
}

func (m *mockRepo) GetRestaurantByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	if m.getRestaurantByIDFunc != nil {
		return m.getRestaurantByIDFunc(ctx, id)
	}
	return &domain.Restaurant{ID: id, WebhookURL: "http://mock-restaurant/webhook", IsActive: true}, nil
}

func (m *mockRepo) UpdateMenuItemAvailability(ctx context.Context, restID, itemID uuid.UUID, isAvailable bool) error {
	if m.updateMenuItemAvailabilityFunc != nil {
		return m.updateMenuItemAvailabilityFunc(ctx, restID, itemID, isAvailable)
	}
	return nil
}

func (m *mockRepo) GetMenuByRestaurantID(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error) {
	if m.getMenuByRestaurantIDFunc != nil {
		return m.getMenuByRestaurantIDFunc(ctx, restID)
	}
	return nil, nil
}

func (m *mockRepo) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	if m.getOrderByIDFunc != nil {
		return m.getOrderByIDFunc(ctx, orderID)
	}
	return nil, errors.New("order not found")
}

func (m *mockRepo) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status domain.OrderStatus) error {
	if m.updateOrderStatusFunc != nil {
		return m.updateOrderStatusFunc(ctx, orderID, status)
	}
	return nil
}

func (m *mockRepo) CreateOrder(ctx context.Context, order *domain.Order, itemRequests []domain.OrderItem) error {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, order, itemRequests)
	}
	return nil
}

func TestService_UpdateOrderStatus_StateMachine(t *testing.T) {
	orderID := uuid.New()

	tests := []struct {
		name          string
		currentStatus domain.OrderStatus
		targetStatus  domain.OrderStatus
		expectedErr   error
	}{
		{
			name:          "Успешный переход: CREATED -> COOKING",
			currentStatus: domain.StatusCreated,
			targetStatus:  domain.StatusCooking,
			expectedErr:   nil,
		},
		{
			name:          "Успешный переход: COOKING -> READY",
			currentStatus: domain.StatusCooking,
			targetStatus:  domain.StatusReady,
			expectedErr:   nil,
		},
		{
			name:          "Успешная отмена: CREATED -> CANCELLED",
			currentStatus: domain.StatusCreated,
			targetStatus:  domain.StatusCancelled,
			expectedErr:   nil,
		},
		{
			name:          "Успешная отмена: COOKING -> CANCELLED",
			currentStatus: domain.StatusCooking,
			targetStatus:  domain.StatusCancelled,
			expectedErr:   nil,
		},
		{
			name:          "Ошибка: прыжок через этап CREATED -> READY",
			currentStatus: domain.StatusCreated,
			targetStatus:  domain.StatusReady,
			expectedErr:   domain.ErrInvalidOrderStatus,
		},
		{
			name:          "Ошибка: откат назад READY -> COOKING",
			currentStatus: domain.StatusReady,
			targetStatus:  domain.StatusCooking,
			expectedErr:   domain.ErrInvalidOrderStatus,
		},
		{
			name:          "Ошибка: нельзя отменить уже готовый заказ READY -> CANCELLED",
			currentStatus: domain.StatusReady,
			targetStatus:  domain.StatusCancelled,
			expectedErr:   domain.ErrInvalidOrderStatus,
		},
		{
			name:          "Ошибка: перевод отмененного заказа CANCELLED -> COOKING",
			currentStatus: domain.StatusCancelled,
			targetStatus:  domain.StatusCooking,
			expectedErr:   domain.ErrInvalidOrderStatus,
		},
		{
			name:          "Ошибка: повторная отмена CANCELLED -> CANCELLED",
			currentStatus: domain.StatusCancelled,
			targetStatus:  domain.StatusCancelled,
			expectedErr:   domain.ErrInvalidOrderStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepo{
				getOrderByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
					return &domain.Order{
						ID:     orderID,
						Status: tt.currentStatus,
					}, nil
				},
				updateOrderStatusFunc: func(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
					return nil
				},
			}

			svc := service.NewService(repo)
			err := svc.UpdateOrderStatus(context.Background(), orderID, tt.targetStatus)

			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("ожидалась ошибка %v, получено %v", tt.expectedErr, err)
			}
		})
	}
}

func TestService_CreateOrder(t *testing.T) {
	restID := uuid.New()
	address := "ул. Ленина, д. 25"
	items := []domain.OrderItem{
		{MenuItemID: uuid.New(), Quantity: 2},
	}

	t.Run("Успешная инициализация и создание заказа", func(t *testing.T) {
		var savedOrder *domain.Order

		repo := &mockRepo{
			createOrderFunc: func(ctx context.Context, order *domain.Order, itemRequests []domain.OrderItem) error {
				savedOrder = order
				return nil
			},
			getRestaurantByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
				return &domain.Restaurant{
					ID:         id,
					WebhookURL: "http://restaurant:8081/webhook/orders",
					IsActive:   true,
				}, nil
			},
		}

		svc := service.NewService(repo)
		created, err := svc.CreateOrder(context.Background(), restID, address, items)

		if err != nil {
			t.Fatalf("неожиданная ошибка: %v", err)
		}
		if created.Status != domain.StatusCreated {
			t.Errorf("ожидался статус %s, получен %s", domain.StatusCreated, created.Status)
		}
		if created.RestaurantID != restID {
			t.Errorf("ожидался ресторан %s, получен %s", restID, created.RestaurantID)
		}
		if created.DeliveryAddress != address {
			t.Errorf("ожидался адрес %s, получен %s", address, created.DeliveryAddress)
		}
		if savedOrder == nil || savedOrder.ID != created.ID {
			t.Errorf("заказ не был передан в репозиторий корректно")
		}
	})

	t.Run("Ошибка репозитория при создании заказа", func(t *testing.T) {
		expectedErr := errors.New("db insert transaction failed")

		repo := &mockRepo{
			createOrderFunc: func(ctx context.Context, order *domain.Order, itemRequests []domain.OrderItem) error {
				return expectedErr
			},
		}

		svc := service.NewService(repo)
		order, err := svc.CreateOrder(context.Background(), restID, address, items)

		if !errors.Is(err, expectedErr) {
			t.Fatalf("ожидалась ошибка %v, получено %v", expectedErr, err)
		}
		if order != nil {
			t.Errorf("заказ должен быть nil при ошибке репозитория")
		}
	})
}
