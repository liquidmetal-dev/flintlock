package transport

import "errors"

var (
	errHandlerRequired      = errors.New("event handler is required")
	errErrorHandlerRequired = errors.New("error handler is required")
	errTopicRequired        = errors.New("topic name is required")
)
