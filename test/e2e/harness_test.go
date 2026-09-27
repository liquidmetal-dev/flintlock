//go:build e2e
// +build e2e

package e2e_test

import (
	"testing"

	. "github.com/onsi/gomega"

	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

func TestPrivateImageRef(t *testing.T) {
	g := NewWithT(t)

	g.Expect(u.PrivateImageRef("ghcr.io/liquidmetal-dev/flintlock-kernel:5.10.77")).
		To(Equal("127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77"))
}

func TestManifestPath(t *testing.T) {
	g := NewWithT(t)

	g.Expect(u.ManifestPath("127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77")).
		To(Equal("/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77"))
}

// The lines are from the log of the registry in a run of the tests, without
// the fields which come after the headers.
const registryLog = `{"time":"2026-09-27T14:54:53.327480622Z","level":"info","message":"HTTP API","module":"http","component":"session","clientIP":"127.0.0.1:45590","method":"GET","path":"/v2/","statusCode":401,"latency":"0s","bodySize":253,"headers":{"Accept":["application/vnd.oci.image.manifest.v1+json"],"Accept-Encoding":["gzip"],"User-Agent":["Go-http-client/1.1"]}}
{"time":"2026-09-27T14:54:55.736062174Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:45614","method":"PUT","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":201,"latency":"0s","bodySize":0,"headers":{"Accept-Encoding":["gzip"],"Authorization":["******"],"Content-Length":["563"],"User-Agent":["skopeo/1.13.3"]}}
{"time":"2026-09-27T14:54:58.875543634Z","level":"info","message":"HTTP API","module":"http","component":"session","clientIP":"127.0.0.1:38398","method":"GET","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":401,"latency":"0s","bodySize":253,"headers":{"Accept":["application/vnd.oci.image.manifest.v1+json"],"Accept-Encoding":["gzip"],"User-Agent":["Go-http-client/1.1"]}}
this line is not JSON
{"time":"2026-09-27T14:54:58.9Z","level":"info","message":"some other message","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":200,"headers":{"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:54:59.020350976Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:38432","method":"HEAD","path":"/v2/liquidmetal-dev/capmvm-k8s-os/manifests/1.23.5","statusCode":200,"latency":"0s","bodySize":0,"headers":{"Authorization":["******"],"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:55:18.1Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:57850","method":"HEAD","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":200,"latency":"0s","bodySize":0,"headers":{"Authorization":["******"],"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:55:18.224102545Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:57850","method":"GET","path":"/v2/liquidmetal-dev/flintlock-kernel/blobs/sha256:ac7c335c24f68ce5f1cae03eabedcecd29326badf138a8e0c8457c710364246b","statusCode":200,"latency":"0s","bodySize":698,"headers":{"Accept":["application/vnd.oci.image.config.v1+json, */*"],"Authorization":["******"],"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:55:19.1Z","level":"info","message":"HTTP API","module":"http","component":"session","clientIP":"127.0.0.1:57850","method":"GET","path":"/v2/liquidmetal-dev/flintlock-kernel-other/manifests/5.10.77","statusCode":404,"latency":"0s","bodySize":0,"headers":{"User-Agent":["containerd/2.2.0+unknown"]}}
`

func TestRequestsFromLog(t *testing.T) {
	const (
		kernelImage  = "127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77"
		kernelBlob   = "/v2/liquidmetal-dev/flintlock-kernel/blobs/sha256:ac7c335c24f68ce5f1cae03eabedcecd29326badf138a8e0c8457c710364246b"
		kernelByTag  = "/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77"
		rootImage    = "127.0.0.1:5050/liquidmetal-dev/capmvm-k8s-os:1.23.5"
		rootByTag    = "/v2/liquidmetal-dev/capmvm-k8s-os/manifests/1.23.5"
		missingImage = "127.0.0.1:5050/liquidmetal-dev/missing:1.0.0"
	)

	tt := []struct {
		name            string
		image           string
		userAgentPrefix string
		expected        []u.RegistryRequest
	}{
		{
			name:            "requests of containerd for the kernel image",
			image:           kernelImage,
			userAgentPrefix: u.ContainerdUserAgent,
			expected: []u.RegistryRequest{
				{Method: "HEAD", Path: kernelByTag, Status: 200},
				{Method: "GET", Path: kernelBlob, Status: 200},
			},
		},
		{
			name:            "requests of containerd for the root image",
			image:           rootImage,
			userAgentPrefix: u.ContainerdUserAgent,
			expected: []u.RegistryRequest{
				{Method: "HEAD", Path: rootByTag, Status: 200},
			},
		},
		{
			name:            "requests of the test itself are not those of containerd",
			image:           kernelImage,
			userAgentPrefix: "Go-http-client/",
			expected: []u.RegistryRequest{
				{Method: "GET", Path: kernelByTag, Status: 401},
			},
		},
		{
			name:            "image which was not requested",
			image:           missingImage,
			userAgentPrefix: u.ContainerdUserAgent,
			expected:        []u.RegistryRequest{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(u.RequestsFromLog([]byte(registryLog), tc.image, tc.userAgentPrefix)).To(Equal(tc.expected))
		})
	}
}

func TestContainerdMajorVersion(t *testing.T) {
	tt := []struct {
		name      string
		output    string
		expected  int
		expectErr bool
	}{
		{
			name:     "v2 release build",
			output:   "containerd github.com/containerd/containerd/v2 v2.2.9 1294c24a7da8e5a793ed378161673abe94118892\n",
			expected: 2,
		},
		{
			name:     "v1 release build",
			output:   "containerd github.com/containerd/containerd v1.7.28 b98a3aace656320842a23f4a392a33f46af97866\n",
			expected: 1,
		},
		{
			name:     "distro build without v prefix or revision",
			output:   "containerd github.com/containerd/containerd/v2 2.2.1-0ubuntu1~24.04.1 \n",
			expected: 2,
		},
		{
			name:      "too few fields",
			output:    "containerd\n",
			expectErr: true,
		},
		{
			name:      "version is not a number",
			output:    "containerd github.com/containerd/containerd/v2 unknown abc\n",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			major, err := u.ContainerdMajorVersion(tc.output)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(major).To(Equal(tc.expected))
		})
	}
}
