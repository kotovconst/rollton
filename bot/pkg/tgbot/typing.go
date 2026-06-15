package tgbot

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var tickInterval = 4 * time.Second

type sendActionFunc func(chatID int64) error

func WithTyping(c *Context, fn func() error) error {
	chatID := c.ChatID()
	if chatID == 0 {
		return fn()
	}
	return withTyping(chatID, defaultSendAction(c.api), fn)
}

func defaultSendAction(api *tgbotapi.BotAPI) sendActionFunc {
	if api == nil {
		return func(int64) error { return nil }
	}
	return func(chatID int64) error {
		_, err := api.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
		return err
	}
}

func withTyping(chatID int64, send sendActionFunc, fn func() error) error {
	_ = send(chatID)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_ = send(chatID)
			}
		}
	}()

	err := fn()
	close(stop)
	<-done
	return err
}
