package tgbot

// HandlerFunc handles a single update.
type HandlerFunc func(*Context) error

// Middleware wraps a HandlerFunc with cross-cutting behavior.
type Middleware func(HandlerFunc) HandlerFunc

// Chain composes middlewares right-to-left around h (outermost first).
func Chain(h HandlerFunc, mws ...Middleware) HandlerFunc {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
