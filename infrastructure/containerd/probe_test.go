package containerd_test

import (
	"context"
	"errors"
	"testing"

	api "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"google.golang.org/genproto/googleapis/rpc/status"

	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

type fakeIntrospection struct {
	plugins []*api.Plugin
	err     error
	filters []string
}

func (f *fakeIntrospection) Plugins(_ context.Context, filters ...string) (*api.PluginsResponse, error) {
	f.filters = filters

	if f.err != nil {
		return nil, f.err
	}

	return &api.PluginsResponse{Plugins: f.plugins}, nil
}

func (f *fakeIntrospection) Server(context.Context) (*api.ServerResponse, error) {
	return &api.ServerResponse{}, nil
}

func (f *fakeIntrospection) PluginInfo(context.Context, string, string, any) (*api.PluginInfoResponse, error) {
	return &api.PluginInfoResponse{}, nil
}

func TestProbeSnapshotter(t *testing.T) {
	errList := errors.New("connection refused")

	tests := []struct {
		name    string
		plugins []*api.Plugin
		listErr error
		wantErr string
	}{
		{
			name: "loaded",
			plugins: []*api.Plugin{
				{Type: "io.containerd.snapshotter.v1", ID: "native"},
				{Type: "io.containerd.snapshotter.v1", ID: "devmapper"},
			},
		},
		{
			name: "skipped with init error",
			plugins: []*api.Plugin{
				{Type: "io.containerd.snapshotter.v1", ID: "devmapper", InitErr: &status.Status{Message: "skip plugin"}},
			},
			wantErr: "snapshotter devmapper: skip plugin: snapshotter is not loaded",
		},
		{
			name:    "absent",
			plugins: []*api.Plugin{{Type: "io.containerd.snapshotter.v1", ID: "overlayfs"}},
			wantErr: "snapshotter devmapper: snapshotter is not loaded",
		},
		{
			name:    "list fails",
			listErr: errList,
			wantErr: "listing containerd snapshotter plugins: connection refused",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)
			introspection := &fakeIntrospection{plugins: tc.plugins, err: tc.listErr}
			client.EXPECT().IntrospectionService().Return(introspection)

			err := containerd.ProbeSnapshotter(context.Background(), &containerd.Config{Namespace: "flintlock"}, client, "devmapper")

			g.Expect(introspection.filters).To(Equal([]string{"type==io.containerd.snapshotter.v1"}))

			if tc.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())

				return
			}

			g.Expect(err).To(MatchError(tc.wantErr))
		})
	}
}
