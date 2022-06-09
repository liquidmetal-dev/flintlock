package auth

import "errors"

var (
	errEmptyAuthToken        = errors.New("empty authentication token")
	errExpectedTokenRequired = errors.New("expected auth token is required")
	errFailedBasicAuth       = errors.New("failed basic authentication. Check the token supplied")
)
