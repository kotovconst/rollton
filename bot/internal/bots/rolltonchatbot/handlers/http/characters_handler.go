package http

import (
	"net/http"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	httpx "github.com/kotovconst/rollton/bot/pkg/http"
)

type CharactersHandler struct {
	svc ports.CharacterService
}

func NewCharactersHandler(svc ports.CharacterService) *CharactersHandler {
	return &CharactersHandler{svc: svc}
}

type characterDTO struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Blurb       string `json:"blurb"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	BotUsername string `json:"bot_username"`
}

func toCharacterDTO(c domain.Character) characterDTO {
	return characterDTO{
		ID:          c.ID.String(),
		Slug:        c.Slug,
		Name:        c.Name,
		Blurb:       c.Blurb,
		AvatarURL:   c.AvatarURL,
		BotUsername: c.BotUsername,
	}
}

func (h *CharactersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListActive(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.ErrCodeInternalError, "list characters failed")
		return
	}
	out := make([]characterDTO, 0, len(list))
	for _, c := range list {
		out = append(out, toCharacterDTO(c))
	}
	httpx.WriteSuccess(w, out)
}
