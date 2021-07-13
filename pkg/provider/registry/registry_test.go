package registry_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/weaveworks/reignite/pkg/provider"
	mock_provider "github.com/weaveworks/reignite/pkg/provider/mock"
	"github.com/weaveworks/reignite/pkg/provider/registry"
)

func TestRegistry_RegisterProvider(t *testing.T) {
	testCases := []struct {
		name              string
		pluginsToRegister []string
		expectRegError    []bool
	}{
		{
			name:              "No plugins registered",
			pluginsToRegister: []string{"prov1"},
			expectRegError:    []bool{false},
		},
		{
			name:              "Plugin already registered",
			pluginsToRegister: []string{"prov1", "prov1"},
			expectRegError:    []bool{false, true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			defer registry.Reset()

			Expect(len(tc.pluginsToRegister)).To(Equal(len(tc.expectRegError)), "length of pluginsToRegister must be the same as expectRegError")

			for i := range tc.pluginsToRegister {
				pluginName := tc.pluginsToRegister[i]
				expectErr := tc.expectRegError[i]

				actualErr := registry.RegisterProvider(pluginName, mockRegisterPluginFactor(false, ctrl))
				if expectErr {
					Expect(actualErr).To(HaveOccurred())
				} else {
					Expect(actualErr).ToNot(HaveOccurred())
				}
			}
		})
	}
}

func TestRegistry_GetInstance(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)

	t.Cleanup(registry.Reset)
	t.Cleanup(ctrl.Finish)

	providerName := "prov1"
	factoryCalled := false

	err := registry.RegisterProvider(providerName, func(ctx context.Context, runtime *provider.Runtime) (provider.MicrovmProvider, error) {
		factoryCalled = true
		return mock_provider.NewMockMicrovmProvider(ctrl), nil
	})
	Expect(err).NotTo(HaveOccurred())

	p, err := registry.GetPluginInstance(context.TODO(), providerName, &provider.Runtime{})
	Expect(err).NotTo(HaveOccurred())
	Expect(p).NotTo(BeNil())
	Expect(factoryCalled).To(BeTrue())
}

func TestRegistry_GetInstanceNotExist(t *testing.T) {
	RegisterTestingT(t)
	ctrl := gomock.NewController(t)

	t.Cleanup(registry.Reset)
	t.Cleanup(ctrl.Finish)

	providerName := "prov1"

	p, err := registry.GetPluginInstance(context.TODO(), providerName, &provider.Runtime{})
	Expect(err).To(HaveOccurred())
	Expect(p).To(BeNil())
}

func TestRegistry_ListAll(t *testing.T) {
	RegisterTestingT(t)

	testCases := []struct {
		name              string
		registeredPlugins []string
		expectedLen       int
	}{
		{
			name:        "No plugins registered",
			expectedLen: 0,
		},
		{
			name:              "Plugins registered",
			registeredPlugins: []string{"prov1", "prov2"},
			expectedLen:       2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			defer registry.Reset()

			for _, pluginName := range tc.registeredPlugins {
				err := registry.RegisterProvider(pluginName, mockRegisterPluginFactor(false, ctrl))
				Expect(err).NotTo(HaveOccurred())
			}

			registered := registry.ListPlugins()
			Expect(registered).To(HaveLen(tc.expectedLen))
		})
	}
}

func mockRegisterPluginFactor(shouldFailCreate bool, ctrl *gomock.Controller) provider.Factory {
	return func(ctx context.Context, runtime *provider.Runtime) (provider.MicrovmProvider, error) {
		if shouldFailCreate {
			return nil, fmt.Errorf("some error occured")
		}

		return mock_provider.NewMockMicrovmProvider(ctrl), nil
	}
}
