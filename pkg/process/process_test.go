package process_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/liquidmetal-dev/flintlock/pkg/process"
	g "github.com/onsi/gomega"
)

func TestSendSignal(t *testing.T) {
	g.RegisterTestingT(t)

	// create a new process
	p := exec.Command("sleep", "10")

	// start the process
	err := p.Start()
	g.Expect(err).NotTo(g.HaveOccurred())

	err = process.SendSignal(p.Process.Pid, os.Kill)
	g.Expect(err).NotTo(g.HaveOccurred())

	// release the pid
	p.Wait()

	// check if process exists
	exists, err := process.Exists(p.Process.Pid)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(exists).To(g.BeFalse())
}

func TestWaitWithContext(t *testing.T) {
	g.RegisterTestingT(t)

	testCases := []struct {
		name    string
		command string
		timeout int
	}{
		{
			name:    "wait on a process that does not exist",
			command: "sleep 5 & echo $! | xargs -I{} kill -9 {}",
			timeout: 5,
		},
		{
			name:    "wait on a process that exists, should succeed",
			command: "sleep 10",
			timeout: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := exec.Command("bash", "-c", tc.command)

			err := p.Start()
			g.Expect(err).NotTo(g.HaveOccurred())

			// wait on the process
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tc.timeout)*time.Second)
			defer cancel()

			// expected to kill the process after timeout
			// as we did not send any signal
			err = process.WaitWithContext(ctx, p.Process.Pid)
			g.Expect(err).NotTo(g.HaveOccurred())

			// check if process exists
			exists, err := process.Exists(p.Process.Pid)
			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(exists).To(g.BeFalse())
		})
	}
}
