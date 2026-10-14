package overlayinit

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseCmdline(t *testing.T) {
	tests := []struct {
		name    string
		cmdline string
		want    Params
		wantErr error
	}{
		{
			name:    "all parameters",
			cmdline: "console=ttyS0 flintlock.root.contract=1 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb init=/lib/systemd/systemd",
			want:    Params{Contract: "1", Base: "/dev/vda", RW: "/dev/vdb", Init: "/lib/systemd/systemd"},
		},
		{
			name:    "init defaults to /sbin/init",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb",
			want:    Params{Contract: "1", Base: "/dev/vda", RW: "/dev/vdb", Init: DefaultInit},
		},
		{
			name:    "order does not matter and bare words are ignored",
			cmdline: "ro flintlock.root.rw=/dev/vdb panic=1 flintlock.root.base=/dev/vda reboot=k flintlock.root.contract=1",
			want:    Params{Contract: "1", Base: "/dev/vda", RW: "/dev/vdb", Init: DefaultInit},
		},
		{
			name:    "quoted values and surrounding whitespace",
			cmdline: "  flintlock.root.contract=\"1\" flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb init=\"/sbin/my init\"\n",
			want:    Params{Contract: "1", Base: "/dev/vda", RW: "/dev/vdb", Init: "/sbin/my init"},
		},
		{
			name:    "last value wins",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=/dev/vdz flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb",
			want:    Params{Contract: "1", Base: "/dev/vda", RW: "/dev/vdb", Init: DefaultInit},
		},
		{
			name:    "missing contract",
			cmdline: "flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb",
			wantErr: ErrMissingParam,
		},
		{
			name:    "unknown contract version",
			cmdline: "flintlock.root.contract=2 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb",
			wantErr: ErrUnknownContract,
		},
		{
			name:    "missing base",
			cmdline: "flintlock.root.contract=1 flintlock.root.rw=/dev/vdb",
			wantErr: ErrMissingParam,
		},
		{
			name:    "missing rw",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=/dev/vda",
			wantErr: ErrMissingParam,
		},
		{
			name:    "empty rw",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=/dev/vda flintlock.root.rw=",
			wantErr: ErrMissingParam,
		},
		{
			name:    "relative device",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=vda flintlock.root.rw=/dev/vdb",
			wantErr: ErrInvalidParam,
		},
		{
			name:    "relative init",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb init=sbin/init",
			wantErr: ErrInvalidParam,
		},
		{
			name:    "base and rw are the same device",
			cmdline: "flintlock.root.contract=1 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vda",
			wantErr: ErrInvalidParam,
		},
		{
			name:    "empty command line",
			cmdline: "",
			wantErr: ErrMissingParam,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := ParseCmdline(tc.cmdline)

			if tc.wantErr != nil {
				g.Expect(err).To(MatchError(tc.wantErr))
				g.Expect(got).To(Equal(Params{}))

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(tc.want))
		})
	}
}

func TestTokenize(t *testing.T) {
	g := NewWithT(t)

	g.Expect(tokenize(`a=1  b="two words" c ""`)).To(Equal([]string{"a=1", "b=two words", "c", ""}))
	g.Expect(tokenize("")).To(BeEmpty())
	g.Expect(tokenize(" \t\n")).To(BeEmpty())
}
