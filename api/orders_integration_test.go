//go:build integration

package api

import (
	"testing"
)

// тестируем полный бизнес-сценарий оформления заказа
func TestSubmitOrder_HappyPath(t *testing.T) {
	testSubmitOrderHappyPath(t)
}

// тестируем оформление заказа с пустым названием книги
func TestSubmitOrder_EmptyBookTitle(t *testing.T) {
	testSubmitOrderEmptyBookTitle(t)
}

// тестируем оформление заказа без услуг
func TestSubmitOrder_NoWorks(t *testing.T) {
	testSubmitOrderNoWorks(t)
}
