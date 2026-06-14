package http

import (
	"net/http"

	"github.com/kotovconst/rollton/bot/internal/middleware"
	httpx "github.com/kotovconst/rollton/bot/pkg/http"
)

type MeHandler struct{}

func NewMeHandler() *MeHandler { return &MeHandler{} }

type meResponse struct {
	User     userDTO     `json:"user"`
	Settings settingsDTO `json:"settings"`
}

type userDTO struct {
	ID         string `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username,omitempty"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name,omitempty"`
}

type settingsDTO struct {
	NotificationsEnabled bool   `json:"notifications_enabled"`
	PreferredLanguage    string `json:"preferred_language"`
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok || u == nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "no user in context")
		return
	}
	httpx.WriteSuccess(w, meResponse{
		User: userDTO{
			ID:         u.ID.String(),
			TelegramID: u.TelegramID,
			Username:   u.Username,
			FirstName:  u.FirstName,
			LastName:   u.LastName,
		},
		Settings: settingsDTO{
			NotificationsEnabled: true,
			PreferredLanguage:    "en",
		},
	})
}
