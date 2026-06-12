// Package http contains rolltonchatbot's HTTP handlers.
package http

import (
	"net/http"

	httpx "github.com/kotovconst/rollton/bot/pkg/http"
)

type HealthzHandler struct{}

func NewHealthzHandler() *HealthzHandler { return &HealthzHandler{} }

func (h *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpx.WriteSuccess(w, map[string]string{"status": "ok", "bot": "rolltonchatbot"})
}
