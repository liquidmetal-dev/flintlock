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
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func createOverlayTestSpec() *models.MicroVM {
	spec := createTestSpec("id1234", "default", testUID)
	spec.Spec.RootVolume.Mode = models.VolumeModeOverlay

	return spec
}

func TestApp_CreateMicroVM_defaultInitrd(t *testing.T) {
	customInitrd := &models.Initrd{
		Image:    "docker.io/library/my-initrd:1",
		Filename: "my-initrd.img",
	}

	testCases := []struct {
		name           string
		specToCreate   *models.MicroVM
		configImage    string
		expectedInitrd *models.Initrd
		expectErr      string
	}{
		{
			name:         "overlay root with a configured image gets it",
			specToCreate: createOverlayTestSpec(),
			configImage:  "ghcr.io/example/overlay-init:custom",
			expectedInitrd: &models.Initrd{
				Image:    "ghcr.io/example/overlay-init:custom",
				Filename: defaults.OverlayInitrdFilename,
			},
		},
		{
			name:         "overlay root with no configured image is rejected",
			specToCreate: createOverlayTestSpec(),
			expectErr:    "--overlay-initrd-image",
		},
		{
			name: "overlay root with an initrd in the spec keeps it",
			specToCreate: func() *models.MicroVM {
				spec := createOverlayTestSpec()
				spec.Spec.Initrd = customInitrd

				return spec
			}(),
			configImage:    "ghcr.io/example/overlay-init:custom",
			expectedInitrd: customInitrd,
		},
		{
			name: "overlay root with an initrd in the spec and no configured image keeps it",
			specToCreate: func() *models.MicroVM {
				spec := createOverlayTestSpec()
				spec.Spec.Initrd = customInitrd

				return spec
			}(),
			expectedInitrd: customInitrd,
		},
		{
			name:           "full root gets no initrd",
			specToCreate:   createTestSpec("id1234", "default", testUID),
			configImage:    "ghcr.io/example/overlay-init:custom",
			expectedInitrd: nil,
		},
		{
			name: "full root with an initrd in the spec keeps it",
			specToCreate: func() *models.MicroVM {
				spec := createTestSpec("id1234", "default", testUID)
				spec.Spec.Initrd = customInitrd

				return spec
			}(),
			expectedInitrd: customInitrd,
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
			ports := &ports.Collection{
				Repo:              rm,
				MicrovmProviders:  map[string]ports.MicroVMService{"mock": pm},
				EventService:      em,
				IdentifierService: im,
				NetworkService:    mock.NewMockNetworkService(mockCtrl),
				ImageService:      mock.NewMockImageService(mockCtrl),
				FileSystem:        afero.NewMemMapFs(),
				Clock:             time.Now,
			}

			pm.EXPECT().Capabilities().Return(models.Capabilities{
				models.MetadataServiceCapability,
				models.MacvtapCapability,
				models.VolumeFullCapability,
				models.VolumeOverlayCapability,
			}).AnyTimes()
			im.EXPECT().GenerateRandom().Return(testUID, nil).Times(1)
			rm.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)

			var saved *models.MicroVM

			if tc.expectErr == "" {
				rm.EXPECT().
					Save(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, mvm *models.MicroVM) (*models.MicroVM, error) {
						saved = mvm

						return mvm, nil
					})
				em.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}

			app := application.New(&application.Config{
				DefaultProvider:    "mock",
				OverlayInitrdImage: tc.configImage,
			}, ports)

			_, err := app.CreateMicroVM(context.Background(), tc.specToCreate)

			if tc.expectErr != "" {
				Expect(err).To(MatchError(ContainSubstring(tc.expectErr)))

				return
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(saved).NotTo(BeNil())
			Expect(saved.Spec.Initrd).To(Equal(tc.expectedInitrd))
		})
	}
}
