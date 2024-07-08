package auth_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/liquidmetal-dev/flintlock/pkg/auth"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
)

func TestBasicAuth_ValidToken(t *testing.T) {
	g := NewWithT(t)

	validToken := "validTokenUnencoded"
	validTokenEncoded := base64.StdEncoding.EncodeToString([]byte(validToken))

	ctx := newIncomingContext(validTokenEncoded)
	authFn := auth.BasicAuthFunc(validToken)

	newCtx, err := authFn(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(newCtx.Value(auth.AuthMethod)).To(Equal("basic"))
	g.Expect(newCtx.Value(auth.Authenticated)).To(BeTrue())
}

func TestBasicAuth_InvalidToken(t *testing.T) {
	g := NewWithT(t)

	validToken := "validTokenUnencoded"
	invalidToken := "invalid"
	invalidTokenEncoded := base64.StdEncoding.EncodeToString([]byte(invalidToken))

	ctx := newIncomingContext(invalidTokenEncoded)
	authFn := auth.BasicAuthFunc(validToken)

	_, err := authFn(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("invalid auth token")))
}

func TestBasicAuth_NoTokenInHeader(t *testing.T) {
	g := NewWithT(t)

	validToken := "validTokenUnencoded"

	ctx := context.Background()
	authFn := auth.BasicAuthFunc(validToken)

	_, err := authFn(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("could not extract token from request header")))
}

func newIncomingContext(token string) context.Context {
	parent := context.Background()
	md := metadata.MD{
		"authorization": []string{"Basic " + token},
	}
	return metadata.NewIncomingContext(parent, md)
}
