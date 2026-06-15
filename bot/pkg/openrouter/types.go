package openrouter

// Role identifies the speaker for an LLM message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a chat conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the input to Client.Complete.
//
// Temperature, TopP, and MaxTokens are pointer fields so nil = "use OpenRouter
// default". This maps cleanly to the NULL columns in model_configs.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
}

// ChatResponse is the result of a successful Complete call.
//
// Reply is the assistant message text (choices[0].message.content).
// FinishReason is OpenRouter's: "stop" | "length" | "content_filter" | ...
type ChatResponse struct {
	Model        string
	Reply        string
	TokensIn     int
	TokensOut    int
	FinishReason string
}
