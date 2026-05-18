//go:build unit

package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"

	"publishing-backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormTruthy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in       string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"yes", true},
		{"on", true},
		{"TRUE", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"no", false},
		{"random", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, formTruthy(tc.in))
		})
	}
}

func newMultipartRequest(t *testing.T, fields map[string]string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	require.NoError(t, w.Close())

	req := httptestNewRequest(t, http.MethodPost, "/works", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(1<<20))
	return req
}

func httptestNewRequest(t *testing.T, method, url string, body *bytes.Buffer) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	return req
}

func TestApplyWorkTextFields_Full(t *testing.T) {
	t.Parallel()

	req := newMultipartRequest(t, map[string]string{
		"description": "описание",
		"work_type":   "печать",
		"unit":        "стр",
	})
	work := &models.Work{Name: "старое", PriceRub: 50}

	err := applyWorkTextFields(req, work, false)
	require.NoError(t, err)
	assert.Equal(t, "описание", work.Description)
	assert.Equal(t, "печать", work.WorkType)
	assert.Equal(t, "стр", work.Unit)
	assert.Equal(t, "старое", work.Name)
	assert.Equal(t, 50, work.PriceRub)
}

func TestApplyWorkTextFields_EmptyName_Partial(t *testing.T) {
	t.Parallel()

	req := newMultipartRequest(t, map[string]string{"name": ""})
	work := &models.Work{Name: "было"}

	err := applyWorkTextFields(req, work, true)
	require.Error(t, err)
	assert.Equal(t, "было", work.Name)
}

func TestApplyWorkTextFields_EmptyPrice_Partial(t *testing.T) {
	t.Parallel()

	// Пустое price_rub не перезаписывает цену (в отличие от "0", которое парсится как 0).
	req := newMultipartRequest(t, map[string]string{"price_rub": ""})
	work := &models.Work{PriceRub: 100}

	err := applyWorkTextFields(req, work, true)
	require.NoError(t, err)
	assert.Equal(t, 100, work.PriceRub)
}

func TestApplyWorkTextFields_Partial(t *testing.T) {
	t.Parallel()

	req := newMultipartRequest(t, map[string]string{"description": "новое"})
	work := &models.Work{Name: "имя", PriceRub: 200, Description: "старое"}

	err := applyWorkTextFields(req, work, true)
	require.NoError(t, err)
	assert.Equal(t, "имя", work.Name)
	assert.Equal(t, 200, work.PriceRub)
	assert.Equal(t, "новое", work.Description)
}
