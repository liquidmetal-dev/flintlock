package config_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/liquidmetal-dev/flintlock/internal/config"
	. "github.com/onsi/gomega"
)

func TestValidateTLSConfig(t *testing.T) {
	g := NewWithT(t)

	tempFile, err := ioutil.TempFile("", "certfile")
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(os.Remove(tempFile.Name())).To(Succeed())
	})

	tt := []struct {
		name      string
		expected  func(*WithT, error)
		tlsConfig config.TLSConfig
	}{
		{
			name: "when all config is valid, no error should occur",
			tlsConfig: config.TLSConfig{
				CertFile:       tempFile.Name(),
				KeyFile:        tempFile.Name(),
				ClientCAFile:   tempFile.Name(),
				ValidateClient: true,
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name: "when CertFile is not set, an error should be returned",
			tlsConfig: config.TLSConfig{
				CertFile: "",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError("certificate file path is required when running securely"))
			},
		},
		{
			name: "when KeyFile is not set, an error should be returned",
			tlsConfig: config.TLSConfig{
				CertFile: "certfile",
				KeyFile:  "",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError("certificate key file path is required when running securely"))
			},
		},
		{
			name: "when the ValidateClient is set but ClientCAFile is not set, an error should be returned",
			tlsConfig: config.TLSConfig{
				CertFile:       "certfile",
				KeyFile:        "keyfile",
				ValidateClient: true,
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("client certificate key file path is required when running mTLS"))
			},
		},
		{
			name: "when the given CertFile is not an existing file, an error should be returned",
			tlsConfig: config.TLSConfig{
				CertFile: "certfile",
				KeyFile:  "keyfile",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("certificate file certfile does not exist"))
			},
		},
		{
			name: "when the given KeyFile is not an existing file, an error should be returned",
			tlsConfig: config.TLSConfig{
				CertFile: tempFile.Name(),
				KeyFile:  "keyfile",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("key file keyfile does not exist"))
			},
		},
		{
			name: "when the ClientCAFile is set and is not an existing file, an error should be returned",
			tlsConfig: config.TLSConfig{
				CertFile:       tempFile.Name(),
				KeyFile:        tempFile.Name(),
				ValidateClient: true,
				ClientCAFile:   "clientcafile",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("client CA file clientcafile does not exist"))
			},
		},
		{
			name: "when Insecure is set, no error should have occurred",
			tlsConfig: config.TLSConfig{
				Insecure: true,
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name: "when Insecure and CertFile are set, an error should be returned",
			tlsConfig: config.TLSConfig{
				Insecure: true,
				CertFile: "certfile",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError("cannot specify tls certificate details when running insecurely"))
			},
		},
		{
			name: "when Insecure and KeyFile are set, an error should be returned",
			tlsConfig: config.TLSConfig{
				Insecure: true,
				KeyFile:  "keyfile",
			},
			expected: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError("cannot specify tls certificate details when running insecurely"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tlsConfig.Validate()
			tc.expected(g, err)
		})
	}
}
