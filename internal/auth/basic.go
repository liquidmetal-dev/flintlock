package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
)

func BasicAuthFunc(expectedToken string) grpc_auth.AuthFunc {
	return func(ctx context.Context) (context.Context, error) {
		token, err := grpc_auth.AuthFromMD(ctx, "Basic")
		if err != nil {
			return nil, err
		}
		if validateErr := validateBasicAuthToken(token, expectedToken); validateErr != nil {
			return nil, validateErr
		}

		newCtx := context.WithValue(ctx, "authenticated", "true")
		newCtx = context.WithValue(newCtx, "auth_method", "basic")

		return newCtx, nil
	}
}

func validateBasicAuthToken(suppliedToken string, expectedToken string) error {
	if expectedToken == "" {
		return errExpectedTokenRequired
	}
	if suppliedToken == "" {
		return errEmptyAuthToken
	}

	data, err := base64.StdEncoding.DecodeString(suppliedToken)
	if err != nil {
		return fmt.Errorf("decoding basic auth token: %w", err)
	}

	if strings.Compare(expectedToken, string(data)) != 0 {
		return errFailedBasicAuth
	}

	return nil
}
