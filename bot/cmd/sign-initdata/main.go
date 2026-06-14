// Helper to generate a valid signed initData string for live testing /api/v1/*.
// Usage: TELEGRAM_TOKEN=... TELEGRAM_USER_ID=... go run ./cmd/sign-initdata
//
// This is a dev-only helper; not deployed. Lives under cmd/ so it can share
// the bot module's deps.
package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "TELEGRAM_TOKEN env required")
		os.Exit(1)
	}
	idStr := os.Getenv("TELEGRAM_USER_ID")
	if idStr == "" {
		idStr = "42"
	}
	id, _ := strconv.ParseInt(idStr, 10, 64)

	now := time.Now()
	userJSON := fmt.Sprintf(`{"id":%d,"first_name":"Alice","username":"alice","language_code":"en","is_premium":false}`, id)
	payload := map[string]string{
		"query_id": "dev-helper",
		"user":     userJSON,
	}
	hash := initdata.Sign(payload, token, now)

	v := url.Values{}
	for k, val := range payload {
		v.Set(k, val)
	}
	v.Set("auth_date", strconv.FormatInt(now.Unix(), 10))
	v.Set("hash", hash)
	fmt.Println(v.Encode())
}
