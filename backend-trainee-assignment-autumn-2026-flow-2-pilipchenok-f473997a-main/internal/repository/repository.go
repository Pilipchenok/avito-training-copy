package repository

import (
	"context"
	"fmt"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"

	"avito-kitchen/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	query := `
		SELECT id, name, webhook_url, is_active 
		FROM restaurants 
		WHERE is_active = true;`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query restaurants: %w", err)
	}
	defer rows.Close()

	var restaurants []domain.Restaurant
	for rows.Next() {
		var rest domain.Restaurant
		if err := rows.Scan(&rest.ID, &rest.Name, &rest.WebhookURL, &rest.IsActive); err != nil {
			return nil, fmt.Errorf("scan restaurant: %w", err)
		}
		restaurants = append(restaurants, rest)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return restaurants, nil
}

func (r *Repository) UpdateMenuItemAvailability(
	ctx context.Context, restID, itemID uuid.UUID, isAvailable bool,
) error {
	query := `
		UPDATE menu_items
		SET is_available = $1
		WHERE id = $2 AND restaurant_id = $3;`

	tag, err := r.pool.Exec(ctx, query, isAvailable, itemID, restID)
	if err != nil {
		return fmt.Errorf("update availability: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrMenuItemNotFound
	}

	return nil
}

func (r *Repository) GetMenuByRestaurantID(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error) {
	query := `
		SELECT id, restaurant_id, name, description, price, is_available
		FROM menu_items
		WHERE restaurant_id = $1;`

	rows, err := r.pool.Query(ctx, query, restID)
	if err != nil {
		return nil, fmt.Errorf("query menu: %w", err)
	}
	defer rows.Close()

	menu := make([]domain.MenuItem, 0)
	for rows.Next() {
		var item domain.MenuItem
		if err := rows.Scan(
			&item.ID,
			&item.RestaurantID,
			&item.Name,
			&item.Description,
			&item.Price,
			&item.IsAvailable,
		); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		menu = append(menu, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("menu rows iteration: %w", err)
	}

	return menu, nil
}

func (r *Repository) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	orderQuery := `
		SELECT *
		FROM orders
		WHERE id = $1;`

	var order domain.Order
	err := r.pool.QueryRow(ctx, orderQuery, orderID).Scan(
		&order.ID,
		&order.RestaurantID,
		&order.Status,
		&order.TotalPrice,
		&order.DeliveryAddress,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("query order: %w", err)
	}

	itemsQuery := `
		SELECT oi.id, oi.order_id, oi.menu_item_id, m.name, oi.quantity, oi.price_at_order
		FROM order_items oi
		JOIN menu_items m ON oi.menu_item_id = m.id
		WHERE oi.order_id = $1;`

	rows, err := r.pool.Query(ctx, itemsQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("query order items: %w", err)
	}
	defer rows.Close()

	order.Items = make([]domain.OrderItem, 0)
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(
			&it.ID,
			&it.OrderID,
			&it.MenuItemID,
			&it.Name,
			&it.Quantity,
			&it.PriceAtOrder,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		order.Items = append(order.Items, it)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("order items rows iteration: %w", err)
	}

	return &order, nil
}

func (r *Repository) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status domain.OrderStatus) error {
	query := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE id = $2;`

	tag, err := r.pool.Exec(ctx, query, status, orderID)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}

	return nil
}

func (r *Repository) CreateOrder(ctx context.Context, order *domain.Order, itemRequests []domain.OrderItem) error {
	if len(itemRequests) == 0 {
		return errors.New("order must contain at least one item")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	requestedIDs := make([]uuid.UUID, 0, len(itemRequests))
	qtyMap := make(map[uuid.UUID]int, len(itemRequests))
	for _, it := range itemRequests {
		requestedIDs = append(requestedIDs, it.MenuItemID)
		qtyMap[it.MenuItemID] += it.Quantity
	}

	queryLock := `
		SELECT id, restaurant_id, name, price, is_available
		FROM menu_items
		WHERE id = ANY($1)
		FOR SHARE;`

	rows, err := tx.Query(ctx, queryLock, requestedIDs)
	if err != nil {
		return fmt.Errorf("lock menu items: %w", err)
	}
	defer rows.Close()

	type dbItem struct {
		id uuid.UUID
		restaurantID uuid.UUID
		name string
		price int
		isAvailable bool
	}
	foundItems := make(map[uuid.UUID]dbItem)

	for rows.Next() {
		var item dbItem
		if err := rows.Scan(&item.id, &item.restaurantID, &item.name, &item.price, &item.isAvailable); err != nil {
			return fmt.Errorf("scan menu item: %w", err)
		}
		foundItems[item.id] = item
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("menu rows error: %w", err)
	}

	var calculatedTotal int
	finalOrderItems := make([]domain.OrderItem, 0, len(itemRequests))

	for itemID, qty := range qtyMap {
		item, exists := foundItems[itemID]
		if !exists || item.restaurantID != order.RestaurantID {
			return domain.ErrMenuItemNotFound
		}

		if !item.isAvailable {
			return domain.ErrMenuItemUnavailable
		}

		calculatedTotal += item.price * qty

		finalOrderItems = append(finalOrderItems, domain.OrderItem{
			ID: uuid.New(),
			OrderID: order.ID,
			MenuItemID: item.id,
			Name: item.name,
			Quantity: qty,
			PriceAtOrder: item.price,
		})
	}

	order.TotalPrice = calculatedTotal
	order.Items = finalOrderItems

	insertOrderQuery := `
		INSERT INTO orders (id, restaurant_id, status, total_price, delivery_address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW());`

	_, err = tx.Exec(ctx, insertOrderQuery,
		order.ID,
		order.RestaurantID,
		order.Status,
		order.TotalPrice,
		order.DeliveryAddress,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	batch := &pgx.Batch{}
	insertItemQuery := `
		INSERT INTO order_items (id, order_id, menu_item_id, quantity, price_at_order)
		VALUES ($1, $2, $3, $4, $5);`

	for _, it := range finalOrderItems {
		batch.Queue(insertItemQuery, it.ID, it.OrderID, it.MenuItemID, it.Quantity, it.PriceAtOrder)
	}

	br := tx.SendBatch(ctx, batch)
	for range finalOrderItems {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("exec batch item: %w", err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("close batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *Repository) GetRestaurantByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	query := `SELECT id, name, webhook_url, is_active FROM restaurants WHERE id = $1;`
	var rest domain.Restaurant
	err := r.pool.QueryRow(ctx, query, id).Scan(&rest.ID, &rest.Name, &rest.WebhookURL, &rest.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRestaurantNotFound
		}
		return nil, fmt.Errorf("query restaurant: %w", err)
	}
	return &rest, nil
}
