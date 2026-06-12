// Package tgbot wraps go-telegram-bot-api with a routing + context layer.
package tgbot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Context carries the active update, the bot client, and request-scoped context.
type Context struct {
	ctx    context.Context
	api    *tgbotapi.BotAPI
	Update tgbotapi.Update
}

func newContext(ctx context.Context, api *tgbotapi.BotAPI, upd tgbotapi.Update) *Context {
	return &Context{ctx: ctx, api: api, Update: upd}
}

// Ctx returns the request-scoped context (cancelled on shutdown).
func (c *Context) Ctx() context.Context { return c.ctx }

// API exposes the raw client for things the wrapper doesn't cover.
func (c *Context) API() *tgbotapi.BotAPI { return c.api }

// UserID returns the sending user's Telegram ID, or 0 if unknown.
func (c *Context) UserID() int64 {
	if u := c.Update.SentFrom(); u != nil {
		return u.ID
	}
	return 0
}

// ChatID returns the chat ID of the update, or 0 if unknown.
func (c *Context) ChatID() int64 {
	if ch := c.Update.FromChat(); ch != nil {
		return ch.ID
	}
	return 0
}

// Reply sends a plain-text reply to the originating chat.
func (c *Context) Reply(text string) error {
	if c.ChatID() == 0 {
		return nil
	}
	msg := tgbotapi.NewMessage(c.ChatID(), text)
	_, err := c.api.Send(msg)
	return err
}

// ReplyMarkdown sends a MarkdownV2-formatted reply to the originating chat.
func (c *Context) ReplyMarkdown(text string) error {
	if c.ChatID() == 0 {
		return nil
	}
	msg := tgbotapi.NewMessage(c.ChatID(), text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err := c.api.Send(msg)
	return err
}
