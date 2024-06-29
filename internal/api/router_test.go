package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projecteru2/vmihub/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := testutils.Prepare(ctx, t)
	assert.Nil(t, err)
	router, err := SetupRouter()
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/healthz", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Language", "en")

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "Healthy", w.Body.String())
}
