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
