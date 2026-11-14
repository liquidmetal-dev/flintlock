package application

import (
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func TestCheckProviderCapabilities_KVMCapabilitiesToDisable(t *testing.T) {
	testCases := []struct {
		name         string
		cpuConfig    *models.CPUConfig
		providerCaps models.Capabilities
		expectErr    error
	}{
		{
			name:         "no cpu config is always allowed",
			cpuConfig:    nil,
			providerCaps: models.Capabilities{},
			expectErr:    nil,
		},
		{
			name:         "kvm_capabilities_to_disable allowed when provider supports it",
			cpuConfig:    &models.CPUConfig{KVMCapabilitiesToDisable: []string{"56"}},
			providerCaps: models.Capabilities{models.KVMCapabilitiesDisableCapability},
			expectErr:    nil,
		},
		{
			name:         "kvm_capabilities_to_disable rejected when provider does not support it",
			cpuConfig:    &models.CPUConfig{KVMCapabilitiesToDisable: []string{"56"}},
			providerCaps: models.Capabilities{},
			expectErr:    errKVMCapabilitiesDisableNotSupported,
		},
		{
			name:         "features_to_enable is never gated",
			cpuConfig:    &models.CPUConfig{FeaturesToEnable: []string{"amx"}},
			providerCaps: models.Capabilities{},
			expectErr:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			provider := mock.NewMockMicroVMService(mockCtrl)
			provider.EXPECT().Capabilities().Return(tc.providerCaps)

			mvm := &models.MicroVM{Spec: models.MicroVMSpec{CPUConfig: tc.cpuConfig}}

			err := checkProviderCapabilities(mvm, provider)
			if tc.expectErr == nil {
				g.Expect(err).NotTo(g.HaveOccurred())
			} else {
				g.Expect(err).To(g.MatchError(tc.expectErr))
			}
		})
	}
}

func TestCheckProviderCapabilities_VolumeMode(t *testing.T) {
	container := func(mode models.VolumeMode) models.Volume {
		return models.Volume{
			ID:   "vol",
			Mode: mode,
			Source: models.VolumeSource{
				Container: &models.ContainerVolumeSource{Image: "docker.io/library/ubuntu:latest"},
			},
		}
	}
	virtioFS := models.Volume{
		ID:     "shared",
		Source: models.VolumeSource{VirtioFS: &models.VirtioFSVolumeSource{Path: "/shared"}},
	}

	both := models.Capabilities{
		models.VirtioFSCapability,
		models.VolumeFullCapability,
		models.VolumeOverlayCapability,
	}
	fullOnly := models.Capabilities{models.VirtioFSCapability, models.VolumeFullCapability}
	overlayOnly := models.Capabilities{models.VirtioFSCapability, models.VolumeOverlayCapability}
	neither := models.Capabilities{models.VirtioFSCapability}

	testCases := []struct {
		name         string
		root         models.Volume
		additional   models.Volumes
		providerCaps models.Capabilities
		expectErr    error
	}{
		{
			name:         "full root needs volume-full",
			root:         container(models.VolumeModeFull),
			providerCaps: fullOnly,
		},
		{
			name:         "unset mode is full and needs volume-full",
			root:         container(""),
			providerCaps: fullOnly,
		},
		{
			name:         "full root without volume-full is rejected",
			root:         container(models.VolumeModeFull),
			providerCaps: overlayOnly,
			expectErr:    errVolumeFullNotSupported,
		},
		{
			name:         "unset mode without volume-full is rejected",
			root:         container(""),
			providerCaps: neither,
			expectErr:    errVolumeFullNotSupported,
		},
		{
			name:         "overlay root needs volume-overlay",
			root:         container(models.VolumeModeOverlay),
			providerCaps: overlayOnly,
		},
		{
			name:         "overlay root without volume-overlay is rejected",
			root:         container(models.VolumeModeOverlay),
			providerCaps: fullOnly,
			expectErr:    errVolumeOverlayNotSupported,
		},
		{
			name:         "overlay root with a full additional volume needs both",
			root:         container(models.VolumeModeOverlay),
			additional:   models.Volumes{container(models.VolumeModeFull)},
			providerCaps: both,
		},
		{
			name:         "overlay root with a full additional volume and no volume-full is rejected",
			root:         container(models.VolumeModeOverlay),
			additional:   models.Volumes{container(models.VolumeModeFull)},
			providerCaps: overlayOnly,
			expectErr:    errVolumeFullNotSupported,
		},
		{
			name:         "overlay root with a virtiofs additional volume needs only volume-overlay",
			root:         container(models.VolumeModeOverlay),
			additional:   models.Volumes{virtioFS},
			providerCaps: overlayOnly,
		},
		{
			name:         "root with no container source needs no volume capability",
			root:         models.Volume{ID: "root"},
			providerCaps: neither,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			provider := mock.NewMockMicroVMService(mockCtrl)
			provider.EXPECT().Capabilities().Return(tc.providerCaps)

			mvm := &models.MicroVM{Spec: models.MicroVMSpec{
				RootVolume:        tc.root,
				AdditionalVolumes: tc.additional,
			}}

			err := checkProviderCapabilities(mvm, provider)
			if tc.expectErr == nil {
				g.Expect(err).NotTo(g.HaveOccurred())
			} else {
				g.Expect(err).To(g.MatchError(tc.expectErr))
			}
		})
	}
}
