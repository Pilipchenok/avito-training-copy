package domain

import (
	"errors"
)

var ErrRestaurantNotFound = errors.New("ресторан не найден")
var ErrMenuItemNotFound = errors.New("позиция не найдена")
var ErrMenuItemUnavailable = errors.New("позиция недоступна")
var ErrInvalidOrderStatus = errors.New("недопустимый статус заказа")
var ErrOrderNotFound = errors.New("заказ не найден")
