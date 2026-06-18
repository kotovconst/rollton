// Package telegram contains character-bot Telegram update handlers.
package telegram

import "strings"

// TelegramMessageMaxBytes is the Telegram sendMessage character cap.
const TelegramMessageMaxBytes = 4096

// ChunkText splits s into chunks no longer than max characters. It prefers
// splitting at the last '\n' within the limit; if none exists, it hard-cuts.
// Empty input returns nil. The returned slice has at least one chunk for any
// non-empty input.
func ChunkText(s string, max int) []string {
	if s == "" {
		return nil
	}
	var out []string
	for len(s) > max {
		cut := strings.LastIndex(s[:max], "\n")
		if cut <= 0 {
			cut = max
			out = append(out, s[:cut])
			s = s[cut:]
			continue
		}
		out = append(out, s[:cut])
		s = s[cut+1:]
	}
	if len(s) > 0 {
		out = append(out, s)
	}
	return out
}
