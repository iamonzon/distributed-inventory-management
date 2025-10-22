package models

import "errors"

// Custom errors for inventory operations
var (
	ErrOutOfStock         = errors.New("insufficient stock available")
	ErrVersionConflict    = errors.New("version conflict - item was modified by another operation")
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")
	ErrItemNotFound       = errors.New("item not found")
	ErrInvalidQuantity    = errors.New("invalid quantity - must be positive")
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
)
