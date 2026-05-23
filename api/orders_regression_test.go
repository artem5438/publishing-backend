//go:build regression

package api

import "testing"

// проверяем полный сценарий оформления заявки с расчётом итоговой цены
func TestSubmitOrder_HappyPath(t *testing.T) {
	testSubmitOrderHappyPath(t)
}

// проверяем, что без названия книги заявку оформить нельзя
func TestSubmitOrder_EmptyBookTitle(t *testing.T) {
	testSubmitOrderEmptyBookTitle(t)
}

// проверяем, что пустую заявку без услуг оформить нельзя
func TestSubmitOrder_NoWorks(t *testing.T) {
	testSubmitOrderNoWorks(t)
}
