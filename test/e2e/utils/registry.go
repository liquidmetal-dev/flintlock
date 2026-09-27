//go:build e2e
// +build e2e

package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gk "github.com/onsi/ginkgo/v2"
	gm "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"golang.org/x/crypto/bcrypt"
)

const (
	registryBin      = "zot"
	skopeoBin        = "skopeo"
	registryHost     = "127.0.0.1"
	registryPort     = "5050"
	registryAddress  = registryHost + ":" + registryPort
	registryUser     = "flintlock"
	registryPassword = "flintlock-e2e" //nolint: gosec // Only for the registry which the test starts.
	ociManifestType  = "application/vnd.oci.image.manifest.v1+json"

	// The registry logs each of the requests with this message.
	registryRequestMessage = "HTTP API"

	// RegistryDir is where the registry keeps its configuration and images
	// during the tests.
	RegistryDir = e2eDataDir + "/registry"

	// ContainerdUserAgent is the start of the User-Agent of the requests which
	// flintlockd makes to a registry, because it pulls the images with the
	// containerd client.
	ContainerdUserAgent = "containerd/"
)

// RegistryRequest is a request which the registry has logged.
type RegistryRequest struct {
	Method string
	Path   string
	Status int
}

type registryLogEntry struct {
	Message    string              `json:"message"`
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
}

// Registry is a private image registry which only allows authenticated access.
type Registry struct {
	dir     string
	session *gexec.Session
}

// NewRegistry creates a new instance of Registry which will keep its
// configuration and images in dir.
func NewRegistry(dir string) *Registry {
	return &Registry{
		dir: dir,
	}
}

// Start writes the registry configuration, starts the registry and waits for
// it to be ready.
// Stop should be called before Start in a defer.
func (r *Registry) Start() {
	htpasswdPath := filepath.Join(r.dir, "htpasswd")
	configPath := filepath.Join(r.dir, "config.json")

	gm.Expect(os.MkdirAll(r.dir, 0o700)).To(gm.Succeed())

	hash, err := bcrypt.GenerateFromPassword([]byte(registryPassword), bcrypt.DefaultCost)
	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Expect(os.WriteFile(htpasswdPath, fmt.Appendf(nil, "%s:%s\n", registryUser, hash), 0o600)).To(gm.Succeed())

	config, err := json.Marshal(map[string]any{
		"distSpecVersion": "1.1.1",
		"storage": map[string]any{
			"rootDirectory": filepath.Join(r.dir, "data"),
		},
		"http": map[string]any{
			"address": registryHost,
			"port":    registryPort,
			"auth": map[string]any{
				"htpasswd": map[string]any{
					"path": htpasswdPath,
				},
			},
		},
		"log": map[string]any{
			"level": "info",
		},
	})
	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Expect(os.WriteFile(configPath, config, 0o600)).To(gm.Succeed())

	log.Printf("TEST INFO: starting the registry on %s with config %s", registryAddress, configPath)

	session, err := gexec.Start(exec.Command(registryBin, "serve", configPath), gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	r.session = session

	// The registry is ready when it answers and asks us to authenticate.
	gm.Eventually(func() (int, error) {
		return getStatus("http://"+registryAddress+"/v2/", false)
	}, "30s", "500ms").Should(gm.Equal(http.StatusUnauthorized))

	log.Printf("TEST INFO: the registry is ready on %s", registryAddress)
}

// Stop stops the registry and removes its configuration and images.
func (r *Registry) Stop() {
	if r.session != nil {
		log.Println("TEST INFO: stopping the registry")
		r.session.Terminate().Wait()
	}

	log.Printf("TEST INFO: removing the registry directory %s", r.dir)

	gm.Expect(os.RemoveAll(r.dir)).To(gm.Succeed())
}

// Seed copies the image to the registry and returns the reference to use to
// pull it from the registry. The copy does not involve containerd, so
// flintlockd will have to pull all of the image from the registry.
func (r *Registry) Seed(image string) string {
	privateImage := PrivateImageRef(image)
	start := time.Now()

	log.Printf("TEST INFO: copying %s to %s", image, privateImage)

	// The registry only accepts OCI manifests.
	//nolint: gosec // The images are chosen by the tests.
	command := exec.Command(skopeoBin, "--insecure-policy", "copy",
		"--format", "oci",
		"--dest-tls-verify=false",
		"--dest-creds", registryUser+":"+registryPassword,
		"docker://"+image,
		"docker://"+privateImage,
	)
	session, err := gexec.Start(command, gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Eventually(session, "10m").Should(gexec.Exit(0))

	log.Printf("TEST INFO: copied %s in %s", privateImage, time.Since(start).Round(time.Millisecond))

	return privateImage
}

// ManifestStatus returns the HTTP status code that the registry responds with
// when asked for the manifest of an image which was returned by Seed.
func (r *Registry) ManifestStatus(privateImage string, authenticated bool) int {
	_, _, found := splitPrivateImage(privateImage)
	gm.Expect(found).To(gm.BeTrue(), "image %s does not have a tag", privateImage)

	url := "http://" + registryAddress + ManifestPath(privateImage)

	status, err := getStatus(url, authenticated)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	access := "anonymous"
	if authenticated {
		access = "authenticated"
	}

	log.Printf("TEST INFO: %s GET %s returned %d", access, url, status)

	return status
}

// Requests returns the requests for the image which the registry has logged
// from the clients with a User-Agent that starts with userAgentPrefix. The
// image must be one which was returned by Seed.
func (r *Registry) Requests(privateImage, userAgentPrefix string) []RegistryRequest {
	gm.Expect(r.session).NotTo(gm.BeNil(), "the registry has not been started")

	registryLog := append(r.session.Out.Contents(), r.session.Err.Contents()...)
	requests := RequestsFromLog(registryLog, privateImage, userAgentPrefix)

	log.Printf("TEST INFO: the registry has logged %d requests for %s from %s",
		len(requests), privateImage, userAgentPrefix)

	for _, request := range requests {
		log.Printf("TEST INFO: %s %s returned %d", request.Method, request.Path, request.Status)
	}

	return requests
}

// RequestsFromLog returns the requests for the image which are in the log of
// the registry, and which are from the clients with a User-Agent that starts
// with userAgentPrefix. The lines of the log which are not the record of a
// request are ignored.
func RequestsFromLog(registryLog []byte, privateImage, userAgentPrefix string) []RegistryRequest {
	requests := []RegistryRequest{}

	repository, _, _ := splitPrivateImage(privateImage)
	pathPrefix := "/v2/" + repository + "/"

	for _, line := range bytes.Split(registryLog, []byte("\n")) {
		var entry registryLogEntry

		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.Message != registryRequestMessage || !strings.HasPrefix(entry.Path, pathPrefix) {
			continue
		}

		if !strings.HasPrefix(http.Header(entry.Headers).Get("User-Agent"), userAgentPrefix) {
			continue
		}

		requests = append(requests, RegistryRequest{
			Method: entry.Method,
			Path:   entry.Path,
			Status: entry.StatusCode,
		})
	}

	return requests
}

// ManifestPath returns the path of the request for the manifest of the image
// by its tag.
func ManifestPath(privateImage string) string {
	repository, tag, _ := splitPrivateImage(privateImage)

	return "/v2/" + repository + "/manifests/" + tag
}

// WriteHostsConfig writes the hosts.toml which flintlockd needs to
// authenticate with the registry. The hostsDir must be the directory that
// flintlockd was started with.
func (r *Registry) WriteHostsConfig(hostsDir string) {
	credentials := base64.StdEncoding.EncodeToString([]byte(registryUser + ":" + registryPassword))
	server := "http://" + registryAddress
	config := fmt.Sprintf(`server = %[1]q

[host.%[1]q]
  capabilities = ["pull", "resolve"]

  [host.%[1]q.header]
    Authorization = "Basic %[2]s"
`, server, credentials)

	hostsPath := filepath.Join(hostsDir, registryAddress, "hosts.toml")

	// The file holds the credentials, so only its path is logged.
	log.Printf("TEST INFO: writing the credentials for %s to %s", server, hostsPath)

	gm.Expect(os.MkdirAll(filepath.Dir(hostsPath), 0o700)).To(gm.Succeed())
	gm.Expect(os.WriteFile(hostsPath, []byte(config), 0o600)).To(gm.Succeed())
}

// PrivateImageRef returns the reference of the image when it is stored in the
// registry.
func PrivateImageRef(image string) string {
	_, path, _ := strings.Cut(image, "/")

	return registryAddress + "/" + path
}

func splitPrivateImage(privateImage string) (string, string, bool) {
	return strings.Cut(strings.TrimPrefix(privateImage, registryAddress+"/"), ":")
}

func getStatus(url string, authenticated bool) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Accept", ociManifestType)

	if authenticated {
		req.SetBasicAuth(registryUser, registryPassword)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	return resp.StatusCode, nil
}
