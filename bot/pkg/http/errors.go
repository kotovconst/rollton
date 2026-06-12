package http

const (
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	ErrCodeInternalError  = "INTERNAL_ERROR"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
)

const (
	MsgInvalidRequestBody  = "Invalid request body"
	MsgInternalServerError = "An internal server error occurred"
	MsgNotFound            = "Resource not found"
	MsgConflict            = "Conflict"
	MsgUnauthorized        = "Unauthorized"
)
