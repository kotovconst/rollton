package tgbot

// Router dispatches updates to registered handlers.
type Router struct {
	commands  map[string]HandlerFunc
	callbacks map[string]HandlerFunc
	fallback  HandlerFunc
}

func NewRouter() *Router {
	return &Router{
		commands:  map[string]HandlerFunc{},
		callbacks: map[string]HandlerFunc{},
	}
}

// Handle registers h for `/command` (without the slash).
func (r *Router) Handle(command string, h HandlerFunc) {
	r.commands[command] = h
}

// HandleCallback registers h for an exact callback_query data match.
func (r *Router) HandleCallback(data string, h HandlerFunc) {
	r.callbacks[data] = h
}

// HandleDefault registers a fallback for unmatched updates.
func (r *Router) HandleDefault(h HandlerFunc) {
	r.fallback = h
}

// Dispatch looks at the update and invokes the matching handler.
func (r *Router) Dispatch(c *Context) error {
	if c.Update.Message != nil && c.Update.Message.IsCommand() {
		if h, ok := r.commands[c.Update.Message.Command()]; ok {
			return h(c)
		}
	}
	if c.Update.CallbackQuery != nil {
		if h, ok := r.callbacks[c.Update.CallbackQuery.Data]; ok {
			return h(c)
		}
	}
	if r.fallback != nil {
		return r.fallback(c)
	}
	return nil
}
