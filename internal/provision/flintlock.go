package provision

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InstallFlintlockd downloads and installs the given (or latest) version of
// flintlockd for arch to InstallPath.
func InstallFlintlockd(ctx context.Context, runner *Runner, version, arch string) error {
	tag, err := ResolveTag(ctx, FlintlockRepo, version)
	if err != nil {
		return fmt.Errorf("resolving flintlockd version: %w", err)
	}

	binName := fmt.Sprintf("%s_%s", FlintlockBin, arch)
	url := DownloadURL(FlintlockRepo, tag, binName)
	dest := filepath.Join(InstallPath, FlintlockBin)

	if err := DownloadFile(ctx, url, dest, 0o755); err != nil { //nolint:gosec // binaries must be executable
		return fmt.Errorf("installing flintlockd %s: %w", tag, err)
	}

	if err := runner.Run(dest, "version"); err != nil {
		return fmt.Errorf("flintlockd %s did not install correctly: %w", tag, err)
	}

	return nil
}

// FlintlockdSettings describes the options used to build a flintlockd config file.
type FlintlockdSettings struct {
	ContainerdSocket string
	Address          string
	Port             string
	ParentIface      string
	BridgeName       string
	Insecure         bool
}

// BuildFlintlockdSettings returns the flintlockd config settings for the
// given options, matching write_flintlockd_config's auto-generated options.
func BuildFlintlockdSettings(s FlintlockdSettings) map[string]string {
	settings := map[string]string{
		"containerd-socket": s.ContainerdSocket,
		"grpc-endpoint":     fmt.Sprintf("%s:%s", s.Address, s.Port),
		"verbosity":         "9",
		"insecure":          fmt.Sprintf("%t", s.Insecure),
	}

	if s.BridgeName != "" {
		settings["bridge-name"] = s.BridgeName
	} else {
		settings["parent-iface"] = s.ParentIface
	}

	return settings
}

// MergeConfigFile merges the "key: value" lines of a user-supplied
// flintlockd config file into settings, overriding any auto-generated
// values with the same key.
func MergeConfigFile(settings map[string]string, configFile string) error {
	f, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("opening %s: %w", configFile, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		mergeConfigLine(settings, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s: %w", configFile, err)
	}

	return nil
}

func mergeConfigLine(settings map[string]string, line string) {
	key, value, found := strings.Cut(line, ":")
	if !found {
		return
	}

	settings[strings.TrimSpace(key)] = strings.ReplaceAll(strings.TrimSpace(value), " ", "")
}

// BuildFlintlockdConfig renders settings as YAML, matching
// write_flintlockd_config's output.
func BuildFlintlockdConfig(settings map[string]string) string {
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var b strings.Builder

	b.WriteString("---\n")

	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, settings[k])
	}

	return b.String()
}

// WriteFlintlockdConfig writes settings, optionally merged with configFile,
// to FlintlockdConfigPath.
func WriteFlintlockdConfig(settings map[string]string, configFile string) error {
	if configFile != "" {
		if err := MergeConfigFile(settings, configFile); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(FlintlockdConfigPath), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(FlintlockdConfigPath), err)
	}

	content := BuildFlintlockdConfig(settings)

	if err := os.WriteFile(FlintlockdConfigPath, []byte(content), 0o644); err != nil { //nolint:gosec // config is not sensitive
		return fmt.Errorf("writing %s: %w", FlintlockdConfigPath, err)
	}

	return nil
}

// StartFlintlockdService fetches the flintlockd systemd unit, points its
// Requires= at the given containerd service, and starts it.
func StartFlintlockdService(ctx context.Context, runner *Runner, containerdSystemdSvc string) error {
	service := filepath.Base(FlintlockdServiceFile)

	if err := FetchServiceFile(ctx, runner, FlintlockRepo, service, FlintlockdServiceFile); err != nil {
		return err
	}

	if err := ReplaceRequires(FlintlockdServiceFile, containerdSystemdSvc); err != nil {
		return err
	}

	return StartService(runner, FlintlockBin)
}

// AllFlintlockOptions bundles the options needed to install, configure and
// start flintlockd, matching do_all_flintlock's parameters.
type AllFlintlockOptions struct {
	Version              string
	Address              string
	ParentIface          string
	BridgeName           string
	Insecure             bool
	ConfigFile           string
	Port                 string
	Arch                 string
	ContainerdStateDir   string
	ContainerdSystemdSvc string
}

// AllFlintlock installs, configures and starts flintlockd, resolving the
// gRPC address/interface/port from the host when they are not supplied,
// matching do_all_flintlock.
func AllFlintlock(ctx context.Context, runner *Runner, opts AllFlintlockOptions) error {
	if err := InstallFlintlockd(ctx, runner, opts.Version, opts.Arch); err != nil {
		return err
	}

	// The interface is resolved whenever it's empty, even when a bridge is
	// supplied - it's still needed below to auto-detect the gRPC address.
	// BuildFlintlockdSettings prefers BridgeName over ParentIface when both
	// are set, so this doesn't affect which one ends up in the config.
	parentIface := opts.ParentIface
	if parentIface == "" {
		iface, err := LookupInterface(runner)
		if err != nil {
			return err
		}

		parentIface = iface
	}

	address := opts.Address
	if address == "" {
		addr, err := LookupAddress(runner, parentIface)
		if err != nil {
			return err
		}

		address = addr
	}

	port := opts.Port
	if port == "" {
		port = "9090"
	}

	settings := BuildFlintlockdSettings(FlintlockdSettings{
		ContainerdSocket: filepath.Join(opts.ContainerdStateDir, "containerd.sock"),
		Address:          address,
		Port:             port,
		ParentIface:      parentIface,
		BridgeName:       opts.BridgeName,
		Insecure:         opts.Insecure,
	})

	if err := WriteFlintlockdConfig(settings, opts.ConfigFile); err != nil {
		return err
	}

	return StartFlintlockdService(ctx, runner, opts.ContainerdSystemdSvc)
}
