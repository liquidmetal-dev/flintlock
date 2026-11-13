package validation

import (
	"testing"

	. "github.com/onsi/gomega"

	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	snapv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
)

func validSnapshotSource() *types.SnapshotSource {
	return &types.SnapshotSource{
		Snapshot: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}},
		Name:     "restored",
	}
}

func TestValidateCreateMicroVMRequest(t *testing.T) {
	tt := []struct {
		name    string
		req     *mvmv1.CreateMicroVMRequest
		wantErr error
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: ErrCreateRequestRequired,
		},
		{
			name: "microvm spec",
			req:  &mvmv1.CreateMicroVMRequest{Microvm: &types.MicroVMSpec{Id: "vm"}},
		},
		{
			name: "snapshot source by uid",
			req:  &mvmv1.CreateMicroVMRequest{SnapshotSource: validSnapshotSource()},
		},
		{
			name: "snapshot source by name",
			req: &mvmv1.CreateMicroVMRequest{SnapshotSource: &types.SnapshotSource{
				Snapshot: &types.SnapshotRef{
					Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "nightly", Namespace: "ns"}},
				},
				Name:       "restored",
				MemoryMode: types.MemoryMode_FILE,
				NetworkOverrides: []*types.HostNetworkOverride{
					{GuestDeviceName: "eth1", HostDeviceName: "tap-restored"},
				},
			}},
		},
		{
			name:    "neither microvm nor snapshot source",
			req:     &mvmv1.CreateMicroVMRequest{},
			wantErr: ErrMicroVMOrSnapshotSource,
		},
		{
			name: "both microvm and snapshot source",
			req: &mvmv1.CreateMicroVMRequest{
				Microvm:        &types.MicroVMSpec{Id: "vm"},
				SnapshotSource: validSnapshotSource(),
			},
			wantErr: ErrMicroVMOrSnapshotSource,
		},
		{
			name: "snapshot source without name",
			req: &mvmv1.CreateMicroVMRequest{SnapshotSource: &types.SnapshotSource{
				Snapshot: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}},
			}},
			wantErr: ErrSnapshotSourceNameRequired,
		},
		{
			name:    "snapshot source without snapshot",
			req:     &mvmv1.CreateMicroVMRequest{SnapshotSource: &types.SnapshotSource{Name: "restored"}},
			wantErr: ErrSnapshotSourceRefRequired,
		},
		{
			name: "snapshot ref with neither uid nor name",
			req: &mvmv1.CreateMicroVMRequest{SnapshotSource: &types.SnapshotSource{
				Snapshot: &types.SnapshotRef{},
				Name:     "restored",
			}},
			wantErr: ErrSnapshotRefInvalid,
		},
		{
			name: "unknown memory mode",
			req: func() *mvmv1.CreateMicroVMRequest {
				source := validSnapshotSource()
				source.MemoryMode = types.MemoryMode(99)

				return &mvmv1.CreateMicroVMRequest{SnapshotSource: source}
			}(),
			wantErr: ErrSnapshotMemoryModeUnknown,
		},
		{
			name: "network override without host device",
			req: func() *mvmv1.CreateMicroVMRequest {
				source := validSnapshotSource()
				source.NetworkOverrides = []*types.HostNetworkOverride{{GuestDeviceName: "eth1"}}

				return &mvmv1.CreateMicroVMRequest{SnapshotSource: source}
			}(),
			wantErr: ErrSnapshotNetworkOverrideName,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			err := ValidateCreateMicroVMRequest(tc.req)
			if tc.wantErr == nil {
				Expect(err).NotTo(HaveOccurred())

				return
			}

			Expect(err).To(MatchError(tc.wantErr))
		})
	}
}

func TestValidateSnapshotRef(t *testing.T) {
	tt := []struct {
		name  string
		ref   *types.SnapshotRef
		valid bool
	}{
		{name: "nil", ref: nil, valid: false},
		{name: "unset", ref: &types.SnapshotRef{}, valid: false},
		{name: "uid", ref: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}}, valid: true},
		{name: "empty uid", ref: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{}}, valid: false},
		{
			name:  "name and namespace",
			ref:   &types.SnapshotRef{Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "n", Namespace: "ns"}}},
			valid: true,
		},
		{
			name:  "name without namespace",
			ref:   &types.SnapshotRef{Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "n"}}},
			valid: false,
		},
		{
			name:  "namespace without name",
			ref:   &types.SnapshotRef{Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Namespace: "ns"}}},
			valid: false,
		},
		{name: "nil named", ref: &types.SnapshotRef{Ref: &types.SnapshotRef_Named{}}, valid: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			err := ValidateSnapshotRef(tc.ref)
			if tc.valid {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(ErrSnapshotRefInvalid))
			}
		})
	}
}

func TestValidateCreateSnapshotRequest(t *testing.T) {
	tt := []struct {
		name    string
		req     *snapv1.CreateSnapshotRequest
		wantErr error
	}{
		{name: "nil request", wantErr: ErrCreateSnapshotRequired},
		{name: "no source uid", req: &snapv1.CreateSnapshotRequest{}, wantErr: ErrSnapshotSourceUIDRequired},
		{
			name:    "empty name",
			req:     &snapv1.CreateSnapshotRequest{SourceUid: "vm-uid", Name: ptr.String("")},
			wantErr: ErrSnapshotNameEmpty,
		},
		{
			name:    "unknown after",
			req:     &snapv1.CreateSnapshotRequest{SourceUid: "vm-uid", After: snapv1.After(7)},
			wantErr: ErrSnapshotAfterUnknown,
		},
		{name: "source uid only", req: &snapv1.CreateSnapshotRequest{SourceUid: "vm-uid"}},
		{
			name: "name, labels and stop",
			req: &snapv1.CreateSnapshotRequest{
				SourceUid: "vm-uid",
				Name:      ptr.String("snap1"),
				Labels:    map[string]string{"env": "test"},
				After:     snapv1.After_STOP,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			err := ValidateCreateSnapshotRequest(tc.req)
			if tc.wantErr == nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(tc.wantErr))
			}
		})
	}
}
