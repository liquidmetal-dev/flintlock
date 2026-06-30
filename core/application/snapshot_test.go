package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/application"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func testSnapshotSpec() *models.MicroVM {
	spec := createTestSpec("id1234", "default", testUID)
	spec.Spec.Provider = "mock"

	return spec
}

func TestApp_SnapshotMicroVM(t *testing.T) {
	frozenTime := time.Now

	testCases := []struct {
		name        string
		uid         string
		reference   string
		expectError bool
		expect      func(
			rm *mock.MockMicroVMRepositoryMockRecorder,
			pm *mock.MockMicroVMServiceMockRecorder,
			sp *mock.MockSnapshotPackagerMockRecorder,
		)
	}{
		{
			name:        "empty uid should fail",
			uid:         "",
			reference:   "myorg/snap:v1",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, pm *mock.MockMicroVMServiceMockRecorder, sp *mock.MockSnapshotPackagerMockRecorder) {
			},
		},
		{
			name:        "empty reference should fail",
			uid:         testUID,
			reference:   "",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, pm *mock.MockMicroVMServiceMockRecorder, sp *mock.MockSnapshotPackagerMockRecorder) {
			},
		},
		{
			name:        "spec not found should fail",
			uid:         testUID,
			reference:   "myorg/snap:v1",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, pm *mock.MockMicroVMServiceMockRecorder, sp *mock.MockSnapshotPackagerMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{UID: testUID}),
				).Return(nil, nil)
			},
		},
		{
			name:        "provider without snapshot capability should fail",
			uid:         testUID,
			reference:   "myorg/snap:v1",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, pm *mock.MockMicroVMServiceMockRecorder, sp *mock.MockSnapshotPackagerMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{UID: testUID}),
				).Return(testSnapshotSpec(), nil)

				pm.Capabilities().Return(models.Capabilities{models.AutoStartCapability})
			},
		},
		{
			name:        "happy path snapshots and packages the image",
			uid:         testUID,
			reference:   "myorg/snap:v1",
			expectError: false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, pm *mock.MockMicroVMServiceMockRecorder, sp *mock.MockSnapshotPackagerMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{UID: testUID}),
				).Return(testSnapshotSpec(), nil)

				pm.Capabilities().Return(models.Capabilities{models.SnapshotCapability})

				pm.Snapshot(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Any(),
				).Return(
					&ports.SnapshotResult{
						Directory: "/scratch",
						Artifacts: []ports.SnapshotArtifact{{Kind: ports.SnapshotMemory, Path: "/scratch/memory"}},
					},
					nil,
				)

				sp.Build(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Any(),
				).Return(
					&ports.SnapshotImage{Reference: "myorg/snap:v1", Digest: "sha256:abc"},
					nil,
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			rm := mock.NewMockMicroVMRepository(mockCtrl)
			em := mock.NewMockEventService(mockCtrl)
			im := mock.NewMockIDService(mockCtrl)
			pm := mock.NewMockMicroVMService(mockCtrl)
			ns := mock.NewMockNetworkService(mockCtrl)
			is := mock.NewMockImageService(mockCtrl)
			sp := mock.NewMockSnapshotPackager(mockCtrl)
			fs := afero.NewMemMapFs()
			ports := &ports.Collection{
				Repo: rm,
				MicrovmProviders: map[string]ports.MicroVMService{
					"mock": pm,
				},
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
				SnapshotPackager:  sp,
			}

			tc.expect(rm.EXPECT(), pm.EXPECT(), sp.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{DefaultProvider: "mock"}, ports)
			image, err := app.SnapshotMicroVM(ctx, tc.uid, tc.reference)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(image.Reference).To(Equal("myorg/snap:v1"))
				Expect(image.Digest).To(Equal("sha256:abc"))
			}
		})
	}
}
