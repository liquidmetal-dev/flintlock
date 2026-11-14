package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	authenticated bool
	authMethod    string
)

const (
	AuthMethod    authMethod    = ""
	Authenticated authenticated = false

	basic = "basic"
)

func BasicAuthFunc(expectedToken string) grpc_auth.AuthFunc {
	return func(ctx context.Context) (context.Context, error) {
		token, err := grpc_auth.AuthFromMD(ctx, basic)
		if err != nil {
			return nil, fmt.Errorf("could not extract token from request header: %w", err)
		}

		if err := validateBasicAuthToken(token, expectedToken); err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
		}

		ctx = context.WithValue(ctx, Authenticated, true)
		ctx = context.WithValue(ctx, AuthMethod, basic)

		return ctx, nil
	}
}

func validateBasicAuthToken(suppliedToken string, expectedToken string) error {
	if expectedToken == "" {
		return errExpectedTokenRequired
	}

	if suppliedToken == "" {
		return errEmptyAuthToken
	}

	data := base64.StdEncoding.EncodeToString([]byte(expectedToken))

	if strings.Compare(suppliedToken, data) != 0 {
		return errFailedBasicAuth
	}

	return nil
}
