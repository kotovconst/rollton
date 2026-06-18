package telegram_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	tgh "github.com/kotovconst/rollton/bot/internal/bots/characterbots/handlers/telegram"
)

func TestChunkText_ShortText_SingleChunk(t *testing.T) {
	chunks := tgh.ChunkText("hello", 4096)
	require.Equal(t, []string{"hello"}, chunks)
}

func TestChunkText_ExactlyLimit_SingleChunk(t *testing.T) {
	s := strings.Repeat("a", 4096)
	require.Equal(t, []string{s}, tgh.ChunkText(s, 4096))
}

func TestChunkText_SplitsAtLastNewline(t *testing.T) {
	first := strings.Repeat("a", 4000) + "\n"
	second := strings.Repeat("b", 200)
	chunks := tgh.ChunkText(first+second, 4096)
	require.Len(t, chunks, 2)
	require.Equal(t, strings.TrimSuffix(first, "\n"), chunks[0])
	require.Equal(t, second, chunks[1])
}

func TestChunkText_NoNewline_HardSplit(t *testing.T) {
	s := strings.Repeat("c", 5000)
	chunks := tgh.ChunkText(s, 4096)
	require.Len(t, chunks, 2)
	require.Equal(t, 4096, len(chunks[0]))
	require.Equal(t, 5000-4096, len(chunks[1]))
}

func TestChunkText_Empty_ReturnsEmpty(t *testing.T) {
	require.Empty(t, tgh.ChunkText("", 4096))
}
