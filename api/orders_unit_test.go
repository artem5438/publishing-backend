//go:build unit

package api

import (
	"testing"

	"publishing-backend/models"

	"github.com/stretchr/testify/assert"
)

// тестируем правила видимости заявки для разных ролей и статусов
func TestOrderVisibleToUser(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		status   models.OrderStatus
		creator  uint
		userID   uint
		role     string
		expected bool
	}{
		{"deleted_hidden", models.StatusDeleted, 1, 1, string(models.RoleCreator), false},
		{"draft_own", models.StatusDraft, 1, 1, string(models.RoleCreator), true},
		{"draft_other", models.StatusDraft, 1, 2, string(models.RoleCreator), false},
		{"formed_own", models.StatusFormed, 1, 1, string(models.RoleCreator), true},
		{"formed_other_creator", models.StatusFormed, 1, 2, string(models.RoleCreator), false},
		{"formed_moderator", models.StatusFormed, 1, 2, string(models.RoleModerator), true},
		{"completed_moderator", models.StatusCompleted, 1, 99, string(models.RoleModerator), true},
		{"completed_other_creator", models.StatusCompleted, 1, 2, string(models.RoleCreator), false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			order := &models.PublishingOrder{
				Status:    tc.status,
				CreatorID: tc.creator,
			}
			got := OrderVisibleToUser(order, tc.userID, tc.role)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// тестируем сортировку работ в заказе
func TestToOrderResponse_WorksSorted(t *testing.T) {
	t.Parallel()

	order := models.PublishingOrder{
		ID:     1,
		Status: models.StatusDraft,
		Works: []models.OrderWork{
			{WorkID: 3, Work: models.Work{ID: 3, Name: "c"}},
			{WorkID: 1, Work: models.Work{ID: 1, Name: "a"}},
			{WorkID: 2, Work: models.Work{ID: 2, Name: "b"}},
		},
	}

	resp := toOrderResponse(order, true)
	requireLen := len(resp.Works)
	if requireLen != 3 {
		t.Fatalf("expected 3 works, got %d", requireLen)
	}
	assert.Equal(t, uint(1), resp.Works[0].WorkID)
	assert.Equal(t, uint(2), resp.Works[1].WorkID)
	assert.Equal(t, uint(3), resp.Works[2].WorkID)
}

// тестируем количество заполненных работ в заказе
func TestToOrderResponse_FilledWorksCount(t *testing.T) {
	t.Parallel()

	order := models.PublishingOrder{
		Status: models.StatusDraft,
		Works: []models.OrderWork{
			{WorkID: 1, Comment: "есть комментарий"},
			{WorkID: 2, Comment: ""},
		},
	}

	resp := toOrderResponse(order, false)
	assert.Equal(t, 1, resp.FilledWorksCount)
}

// тестируем копирование общей стоимости заказа
func TestToOrderResponse_CopiesTotalPrice(t *testing.T) {
	t.Parallel()

	total := 5000
	order := models.PublishingOrder{
		Status:     models.StatusFormed,
		TotalPrice: &total,
	}

	resp := toOrderResponse(order, false)
	assert.NotNil(t, resp.TotalPrice)
	assert.Equal(t, 5000, *resp.TotalPrice)
}
