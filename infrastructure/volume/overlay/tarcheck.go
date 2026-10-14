package overlay

import (
	"archive/tar"
	"io"
	"path"
	"strings"
)

// hardlinkCheck watches a tar stream for the first hardlink entry whose
// target is not an earlier entry of the same stream. mkfs.erofs builds each
// layer on its own, so such an entry cannot be resolved: its target is in a
// lower layer or is absent. The check runs beside mkfs.erofs on a tee of the
// stream and does not change the bytes mkfs.erofs reads.
type hardlinkCheck struct {
	pr   *io.PipeReader
	pw   *io.PipeWriter
	done chan struct{}

	entry  string
	target string
	found  bool
}

func newHardlinkCheck() *hardlinkCheck {
	pr, pw := io.Pipe()
	c := &hardlinkCheck{pr: pr, pw: pw, done: make(chan struct{})}

	go c.run()

	return c
}

// reader returns r with every byte read also fed to the check.
func (c *hardlinkCheck) reader(r io.Reader) io.Reader {
	return io.TeeReader(r, c.pw)
}

// run parses the tee until the stream ends. It drains the pipe after a
// parse error so that the tee never blocks.
func (c *hardlinkCheck) run() {
	defer close(c.done)
	defer func() { _, _ = io.Copy(io.Discard, c.pr) }()

	seen := map[string]struct{}{}
	tr := tar.NewReader(c.pr)

	for {
		hdr, err := tr.Next()
		if err != nil {
			return
		}

		name := cleanTarPath(hdr.Name)

		if hdr.Typeflag == tar.TypeLink && !c.found {
			target := cleanTarPath(hdr.Linkname)
			if _, ok := seen[target]; !ok {
				c.entry, c.target, c.found = name, target, true
			}
		}

		seen[name] = struct{}{}
	}
}

// finish ends the check and reports the first foreign hardlink.
func (c *hardlinkCheck) finish() (entry, target string, found bool) {
	_ = c.pw.Close()
	<-c.done

	return c.entry, c.target, c.found
}

// cleanTarPath normalises a tar entry name so that "./a", "/a" and "a" are
// one path.
func cleanTarPath(name string) string {
	return path.Clean("/" + strings.TrimPrefix(name, "/"))[1:]
}

// drain reads the rest of r through the check so that the check sees the
// whole layer when mkfs.erofs stopped early.
func (c *hardlinkCheck) drain(r io.Reader) {
	_, _ = io.Copy(io.Discard, c.reader(r))
}
