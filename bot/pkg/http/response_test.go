package http_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	httpx "github.com/kotovconst/rollton/bot/pkg/http"
	"github.com/stretchr/testify/require"
)

func TestWriteSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteSuccess(rec, map[string]string{"hello": "world"})

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got httpx.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.True(t, got.Success)
	require.Nil(t, got.Error)
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, 400, httpx.ErrCodeInvalidRequest, "bad body")

	require.Equal(t, 400, rec.Code)
	var got httpx.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.False(t, got.Success)
	require.Equal(t, httpx.ErrCodeInvalidRequest, got.Error.Code)
	require.Equal(t, "bad body", got.Error.Message)
}
