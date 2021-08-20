package containerd_test

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/containerd"
	"github.com/weaveworks/reignite/pkg/defaults"

	ctr "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
)

var (
	testImage          = "docker.io/library/alpine:3.14.1"
	testSnapshotter    = "native"
	testOwnerNamespace = "int_ns"
	testOwnerName      = "imageservice-get-test"

	//go:embed testdata/config.toml
	containerdConfig string
)

func TestImageService_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd image service integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)
	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	imageSvc := containerd.NewImageServiceWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
	}, client)

	input := ports.GetImageInput{
		ImageName:      testImage,
		OwnerName:      testOwnerName,
		OwnerNamespace: testOwnerNamespace,
		Use:            models.ImageUseVolume,
	}
	err := imageSvc.Get(ctx, input)
	Expect(err).NotTo(HaveOccurred())

	mounts, err := imageSvc.GetAndMount(ctx, input)
	Expect(err).NotTo(HaveOccurred())
	Expect(mounts).NotTo(BeNil())
	Expect(len(mounts)).To(Equal(1))

	img, err := client.ImageService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(img)).To(Equal(1))
	Expect(img[0].Name).To(Equal(testImage))

	expectedSnapshotName := fmt.Sprintf("reignite/%s", testOwnerName)
	snapshotExists := false
	err = client.SnapshotService(testSnapshotter).Walk(namespaceCtx, func(walkCtx context.Context, info snapshots.Info) error {
		if info.Name == expectedSnapshotName {
			snapshotExists = true
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(snapshotExists).To(BeTrue(), "expect snapshot with name %s to exist", expectedSnapshotName)

	expectedLeaseName := fmt.Sprintf("reignite/%s/%s", testOwnerNamespace, testOwnerName)
	leases, err := client.LeasesService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(leases)).To(Equal(1))
	Expect(leases[0].ID).To(Equal(expectedLeaseName), "expect lease with name %s to exists", expectedLeaseName)

	input.Use = models.ImageUseKernel
	input.ImageName = "docker.io/linuxkit/kernel:5.4.129"

	err = imageSvc.Get(ctx, input)
	Expect(err).NotTo(HaveOccurred())
}

func TestMain(m *testing.M) {
	if !runContainerDTests() {
		os.Exit(m.Run())
	}

	rootDir := os.Getenv("CTR_ROOT_DIR")
	if err := os.RemoveAll(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "could not empty test folder %s: %s\n", rootDir, err)
		os.Exit(1)
	}
	fmt.Printf("the containerd root folder is %s\n", rootDir)

	cleanup, err := startContainerd(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start containerd: %s\n", err)
		os.Exit(1)
	}

	status := m.Run()

	cleanup()
	os.Exit(status)
}

func testCreateClient(t *testing.T) (*ctr.Client, context.Context) {
	addr := containerDTestSocketPath()
	client, err := ctr.New(addr)
	Expect(err).NotTo(HaveOccurred())

	ctx := context.Background()

	serving, err := client.IsServing(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(serving).To(BeTrue())

	return client, ctx
}

func runContainerDTests() bool {
	testCtr := os.Getenv("CTR_ROOT_DIR")
	return testCtr != ""
}

func containerDTestSocketPath() string {
	rootDir := os.Getenv("CTR_ROOT_DIR")
	return fmt.Sprintf("%s/containerd.sock", rootDir)
}

func startContainerd(rootDir string) (func(), error) {
	root := fmt.Sprintf("%s/root", rootDir)
	os.MkdirAll(root, os.ModePerm)
	state := fmt.Sprintf("%s/state", rootDir)
	os.MkdirAll(state, os.ModePerm)
	addr := containerDTestSocketPath()
	cfg := fmt.Sprintf("%s/containerd.config", rootDir)

	stdOutFile, err := os.Create(fmt.Sprintf("%s/stdout.txt", rootDir))
	if err != nil {
		return nil, fmt.Errorf("could not open containerd stdout file file %s: %w", stdOutFile.Name(), err)
	}
	stdErrFile, err := os.Create(fmt.Sprintf("%s/stderr.txt", rootDir))
	if err != nil {
		return nil, fmt.Errorf("could not open containerd stderr file file %s: %w", stdErrFile.Name(), err)
	}

	if err := writeContainerdConfig(cfg); err != nil {
		return nil, fmt.Errorf("writing containerd config file: %w", err)
	}

	args := []string{
		"--address",
		addr,
		"--root",
		root,
		"--state",
		state,
		"--log-level",
		"debug",
		"--config",
		cfg,
	}
	cmd := exec.Command("containerd", args...)
	cmd.Stdout = stdOutFile
	cmd.Stderr = stdErrFile
	if err := cmd.Start(); err != nil {
		cmd.Wait()
		return nil, fmt.Errorf("failed to start containerd: %w", err)
	}

	cleanup := func() {
		stdOutFile.Close()
		stdErrFile.Close()
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Process.Wait()
	}

	return cleanup, nil
}

func writeContainerdConfig(configPath string) error {
	cfgFile, err := os.Create(configPath)
	defer cfgFile.Close()

	if err != nil {
		return fmt.Errorf("could not open containerd config file %s: %w", configPath, err)
	}
	if _, err := cfgFile.WriteString(containerdConfig); err != nil {
		return fmt.Errorf("Failed to write to config file %s: %w", configPath, err)
	}

	return nil
}
