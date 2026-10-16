CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS restaurants (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name VARCHAR(255) NOT NULL,
  webhook_url VARCHAR(512) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS menu_items (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  description TEXT DEFAULT '',
  price INT NOT NULL CHECK (price >= 0),
  is_available BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_menu_items_restaurant_available 
ON menu_items(restaurant_id, is_available);

CREATE TABLE IF NOT EXISTS orders (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE RESTRICT,
  status VARCHAR(32) NOT NULL DEFAULT 'CREATED',
  total_price INT NOT NULL CHECK (total_price >= 0),
  delivery_address TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_restaurant_status 
ON orders(restaurant_id, status);

CREATE TABLE IF NOT EXISTS order_items (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE RESTRICT,
  quantity INT NOT NULL CHECK (quantity > 0),
  price_at_order INT NOT NULL CHECK (price_at_order >= 0)
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id 
ON order_items(order_id);

-- Начальные данные

INSERT INTO restaurants (id, name, webhook_url, is_active)
VALUES (
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  'Додо Пицца',
  'http://restaurant:8081/webhook/orders',
  true
) ON CONFLICT (id) DO NOTHING;

INSERT INTO menu_items (id, restaurant_id, name, description, price, is_available)
VALUES 
  (
    'b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Пицца Пепперони',
    'Классическая пицца с пикантной пепперони и моцареллой',
    599,
    true
  ),
  (
    'c2eebc99-9c0b-4ef8-bb6d-6bb9bd380a33',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Пицца Четыре Сыра',
    'Сырный соус, моцарелла, пармезан, чеддер и блю чиз',
    649,
    true
  ),
  (
    'd3eebc99-9c0b-4ef8-bb6d-6bb9bd380a44',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Морс Клюквенный',
    'Освежающий морс из натуральной клюквы 0.5л',
    149,
    true
  )
ON CONFLICT (id) DO NOTHING;
