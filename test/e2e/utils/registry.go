//go:build e2e
// +build e2e

package utils

import (
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
	registryPassword = "flintlock-e2e"
	ociManifestType  = "application/vnd.oci.image.manifest.v1+json"

	// RegistryDir is where the registry keeps its configuration and images
	// during the tests.
	RegistryDir = e2eDataDir + "/registry"
)

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
	repository, tag, found := strings.Cut(strings.TrimPrefix(privateImage, registryAddress+"/"), ":")
	gm.Expect(found).To(gm.BeTrue(), "image %s does not have a tag", privateImage)

	url := fmt.Sprintf("http://%s/v2/%s/manifests/%s", registryAddress, repository, tag)

	status, err := getStatus(url, authenticated)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	access := "anonymous"
	if authenticated {
		access = "authenticated"
	}

	log.Printf("TEST INFO: %s GET %s returned %d", access, url, status)

	return status
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

func getStatus(url string, authenticated bool) (int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
