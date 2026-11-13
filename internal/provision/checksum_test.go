package provision_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

const assetName = "mkfs.erofs-1.9.4-linux-amd64"

// newAssetServer serves content at /<assetName> and sumLine at
// /<assetName>.sha256. A nil sumLine means no sha256 file is published.
func newAssetServer(t *testing.T, content []byte, sumLine *string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	})

	mux.HandleFunc("/"+assetName+".sha256", func(w http.ResponseWriter, _ *http.Request) {
		if sumLine == nil {
			http.NotFound(w, nil)

			return
		}

		_, _ = w.Write([]byte(*sumLine))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)

	return hex.EncodeToString(sum[:])
}

func TestDownloadVerified(t *testing.T) {
	content := []byte("#!/bin/sh\necho 'mkfs.erofs (erofs-utils) 1.9.4'\n")
	goodSum := sha256Hex(content) + "  " + assetName + "\n"
	badSum := sha256Hex([]byte("other")) + "  " + assetName + "\n"
	malformedSum := "<html><body>Not Found</body></html>"

	tests := []struct {
		name      string
		sumLine   *string
		wantErr   error
		installed bool
	}{
		{name: "matching digest installs the file", sumLine: &goodSum, installed: true},
		{name: "wrong digest stops the install", sumLine: &badSum, wantErr: provision.ErrChecksumMismatch},
		{name: "missing sha256 file stops the install", sumLine: nil},
		{name: "malformed sha256 file stops the install", sumLine: &malformedSum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			srv := newAssetServer(t, content, tt.sumLine)
			dest := filepath.Join(t.TempDir(), "bin", "mkfs.erofs")
			url := srv.URL + "/" + assetName

			err := provision.DownloadVerified(context.Background(), url, url+".sha256", dest, 0o755)

			if tt.installed {
				g.Expect(err).NotTo(HaveOccurred())

				got, readErr := os.ReadFile(dest)
				g.Expect(readErr).NotTo(HaveOccurred())
				g.Expect(got).To(Equal(content))

				info, statErr := os.Stat(dest)
				g.Expect(statErr).NotTo(HaveOccurred())
				g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o755)))

				return
			}

			g.Expect(err).To(HaveOccurred())

			if tt.wantErr != nil {
				g.Expect(err).To(MatchError(tt.wantErr))
			}

			_, statErr := os.Stat(dest)
			g.Expect(os.IsNotExist(statErr)).To(BeTrue(), "nothing may be installed at %s", dest)

			entries, _ := os.ReadDir(filepath.Dir(dest))
			g.Expect(entries).To(BeEmpty(), "no temp file may be left behind")
		})
	}
}

func TestDownloadVerified_MissingAsset(t *testing.T) {
	g := NewWithT(t)

	sum := sha256Hex([]byte("x")) + "  " + assetName + "\n"
	srv := newAssetServer(t, []byte("x"), &sum)
	dest := filepath.Join(t.TempDir(), "mkfs.erofs")
	url := srv.URL + "/does-not-exist"

	err := provision.DownloadVerified(context.Background(), url, srv.URL+"/"+assetName+".sha256", dest, 0o755)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("404"))
}

func TestParseSHA256Sum(t *testing.T) {
	digest := sha256Hex([]byte("content"))

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{name: "sha256sum line", content: digest + "  " + assetName + "\n", want: digest},
		{name: "digest only", content: digest, want: digest},
		{name: "upper case digest", content: "ABCDEF" + digest[6:], want: "abcdef" + digest[6:]},
		{name: "empty", content: "", wantErr: true},
		{name: "too short", content: "abc  file\n", wantErr: true},
		{name: "not hex", content: "zz" + digest[2:], wantErr: true},
		{name: "html error page", content: "<html><body>Not Found</body></html>", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := provision.ParseSHA256Sum(tt.content)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(tt.want))
		})
	}
}
