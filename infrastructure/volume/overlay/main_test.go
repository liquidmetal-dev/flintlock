package overlay_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake mkfs.erofs. The builder runs it with
// fakeMkfsEnv set. The fake records every command line in the file named
// by fakeMkfsLogEnv and writes predictable output files:
//
//   - a layer build reads stdin to the end and writes
//     "layer <uuid>\n<sha256 of stdin>\n";
//   - a merge writes "merge <uuid>\n" followed by the input files.
//
// fakeMkfsVersionEnv sets the version -V reports. fakeMkfsFailEnv makes a
// "layer" or "merge" run exit 1, and "layer-early" makes a layer run stop
// after one tar block, as mkfs.erofs does at a bad entry. fakeMkfsSleepEnv
// delays a merge. A command line with -L is mkfs.ext4, see fakeMkfsExt4.
const (
	fakeMkfsEnv        = "FLINTLOCK_TEST_FAKE_MKFS"
	fakeMkfsLogEnv     = "FLINTLOCK_TEST_FAKE_MKFS_LOG"
	fakeMkfsVersionEnv = "FLINTLOCK_TEST_FAKE_MKFS_VERSION"
	fakeMkfsFailEnv    = "FLINTLOCK_TEST_FAKE_MKFS_FAIL"
	fakeMkfsSleepEnv   = "FLINTLOCK_TEST_FAKE_MKFS_SLEEP"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeMkfsEnv) == "1" {
		os.Exit(fakeMkfs(os.Args[1:]))
	}

	os.Exit(m.Run())
}

func fakeMkfs(args []string) int {
	if len(args) == 1 && args[0] == "-V" {
		version := os.Getenv(fakeMkfsVersionEnv)
		if version == "" {
			version = "1.9.4"
		}

		fmt.Printf("mkfs.erofs (erofs-utils) %s\navailable compressors: \n", version)

		return 0
	}

	if logPath := os.Getenv(fakeMkfsLogEnv); logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)

			return 2
		}

		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}

	if slices.Contains(args, "-L") {
		return fakeMkfsExt4(args)
	}

	mode := "merge"
	if slices.Contains(args, "--tar=f") {
		mode = "layer"
	}

	switch fail := os.Getenv(fakeMkfsFailEnv); {
	case fail == mode:
		fmt.Fprintf(os.Stderr, "fake mkfs.erofs: %s failed\n", mode)

		return 1
	case fail == "layer-early" && mode == "layer":
		_, _ = io.ReadFull(os.Stdin, make([]byte, 512))
		fmt.Fprintln(os.Stderr, "fake mkfs.erofs: No such file or directory")

		return 1
	}

	if d := os.Getenv(fakeMkfsSleepEnv); d != "" && mode == "merge" {
		sleep, err := time.ParseDuration(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)

			return 2
		}

		time.Sleep(sleep)
	}

	u := slices.Index(args, "-U")
	if u < 0 || len(args) < u+4 {
		fmt.Fprintln(os.Stderr, "fake mkfs.erofs: bad arguments")

		return 2
	}

	fsUUID, out, inputs := args[u+1], args[u+2], args[u+3:]

	var content bytes.Buffer

	fmt.Fprintf(&content, "%s %s\n", mode, fsUUID)

	if mode == "layer" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)

			return 2
		}

		sum := sha256.Sum256(data)
		fmt.Fprintln(&content, hex.EncodeToString(sum[:]))
	} else {
		for _, in := range inputs {
			b, err := os.ReadFile(in)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)

				return 1
			}

			content.Write(b)
		}
	}

	if err := os.WriteFile(out, content.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return 0
}

// fakeMkfsExt4 is the fake mkfs.ext4. It writes "ext4 <uuid> <label>\n" at
// the start of the file named last, and keeps the size of the file.
// fakeMkfsFailEnv set to "ext4" makes it exit 1, and fakeMkfsSleepEnv
// delays it.
func fakeMkfsExt4(args []string) int {
	if os.Getenv(fakeMkfsFailEnv) == "ext4" {
		fmt.Fprintln(os.Stderr, "fake mkfs.ext4: failed")

		return 1
	}

	if d := os.Getenv(fakeMkfsSleepEnv); d != "" {
		sleep, err := time.ParseDuration(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)

			return 2
		}

		time.Sleep(sleep)
	}

	u := slices.Index(args, "-U")
	l := slices.Index(args, "-L")

	if u < 0 || l < 0 || len(args) < u+2 || len(args) < l+2 {
		fmt.Fprintln(os.Stderr, "fake mkfs.ext4: bad arguments")

		return 2
	}

	f, err := os.OpenFile(args[len(args)-1], os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	defer f.Close()

	if _, err := fmt.Fprintf(f, "ext4 %s %s\n", args[u+1], args[l+1]); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return 0
}
