package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ContainerdPaths holds the various state paths used by containerd. A
// "-dev" tagged set is used in development environments so a dev
// containerd instance never collides with a production one on the same host.
type ContainerdPaths struct {
	ConfigPath   string
	RootDir      string
	StateDir     string
	ServiceFile  string
	SystemdSvc   string
	DevMapperDir string
	PoolMetadata string
	PoolData     string
}

// BuildContainerdPaths returns the containerd state paths to use, tagging
// them with "-dev" when dev is true.
func BuildContainerdPaths(dev bool) ContainerdPaths {
	tag := ""
	if dev {
		tag = "-dev"
	}

	rootDir := fmt.Sprintf("/var/lib/containerd%s", tag)
	devMapperDir := filepath.Join(rootDir, "snapshotter", "devmapper")

	return ContainerdPaths{
		ConfigPath:   fmt.Sprintf("/etc/containerd/config%s.toml", tag),
		RootDir:      rootDir,
		StateDir:     fmt.Sprintf("/run/containerd%s", tag),
		ServiceFile:  fmt.Sprintf("/etc/systemd/system/containerd%s.service", tag),
		SystemdSvc:   fmt.Sprintf("containerd%s.service", tag),
		DevMapperDir: devMapperDir,
		PoolMetadata: filepath.Join(devMapperDir, "metadata"),
		PoolData:     filepath.Join(devMapperDir, "data"),
	}
}

// MakeContainerdDirs creates the directories containerd needs before it can start.
func MakeContainerdDirs(paths ContainerdPaths) error {
	dirs := []string{paths.DevMapperDir, paths.StateDir, filepath.Dir(paths.ConfigPath)}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating containerd directory %s: %w", dir, err)
		}
	}

	return nil
}

// InstallContainerd downloads and installs the given (or latest) version of
// containerd for arch to InstallPath.
func InstallContainerd(ctx context.Context, runner *Runner, version, arch string) error {
	tag, err := ResolveTag(ctx, ContainerdRepo, version)
	if err != nil {
		return fmt.Errorf("resolving containerd version: %w", err)
	}

	binName := ContainerdReleaseBinName(tag, arch)
	url := DownloadURL(ContainerdRepo, tag, binName)

	if err := ExtractTarGz(ctx, url, filepath.Dir(InstallPath)); err != nil {
		return fmt.Errorf("installing containerd %s: %w", tag, err)
	}

	dest := filepath.Join(InstallPath, ContainerdBin)
	if err := runner.Run(dest, "--version"); err != nil {
		return fmt.Errorf("containerd %s did not install correctly: %w", tag, err)
	}

	return nil
}

// BuildContainerdConfig renders the containerd config.toml content for the
// given paths and thinpool name.
func BuildContainerdConfig(paths ContainerdPaths, thinpool string) string {
	return fmt.Sprintf(`version = 2

root = "%s"
state = "%s"

[grpc]
  address = "%s"

[metrics]
  address = "127.0.0.1:1338"

[plugins]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "%s-thinpool"
    root_path = "%s"
    base_image_size = "10GB"
    discard_blocks = true

[debug]
  level = "trace"
`, paths.RootDir, paths.StateDir, filepath.Join(paths.StateDir, "containerd.sock"), thinpool, paths.DevMapperDir)
}

// WriteContainerdConfig writes the containerd config.toml for the given paths and thinpool.
func WriteContainerdConfig(paths ContainerdPaths, thinpool string) error {
	content := BuildContainerdConfig(paths, thinpool)

	if err := os.WriteFile(paths.ConfigPath, []byte(content), 0o644); err != nil { //nolint:gosec // config is not sensitive
		return fmt.Errorf("writing containerd config %s: %w", paths.ConfigPath, err)
	}

	return nil
}

// StartContainerdService fetches the containerd systemd unit, points it at
// paths.ConfigPath and starts it.
func StartContainerdService(ctx context.Context, runner *Runner, paths ContainerdPaths) error {
	if err := FetchServiceFile(ctx, runner, ContainerdRepo, fmt.Sprintf("%s.service", ContainerdBin), paths.ServiceFile); err != nil {
		return err
	}

	if err := appendExecStartArg(paths.ServiceFile, "--config "+paths.ConfigPath); err != nil {
		return err
	}

	return StartService(runner, paths.SystemdSvc)
}

// AllContainerd installs, configures and starts containerd for the given
// version and thinpool, tagging state paths with "-dev" when dev is true.
func AllContainerd(ctx context.Context, runner *Runner, version, thinpool, arch string, dev bool) (ContainerdPaths, error) {
	paths := BuildContainerdPaths(dev)

	if err := MakeContainerdDirs(paths); err != nil {
		return paths, err
	}

	if err := InstallContainerd(ctx, runner, version, arch); err != nil {
		return paths, err
	}

	if err := WriteContainerdConfig(paths, thinpool); err != nil {
		return paths, err
	}

	if err := StartContainerdService(ctx, runner, paths); err != nil {
		return paths, err
	}

	return paths, nil
}
