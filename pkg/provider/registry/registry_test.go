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
	RegisterTestingT(t)
	t.Cleanup(registry.Reset)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	err := registry.RegisterProvider("prov1", mockRegisterPluginFactor(false, ctrl))
	Expect(err).NotTo(HaveOccurred())
}

func TestRegistry_RegisterProviderDuplicate(t *testing.T) {
	RegisterTestingT(t)
	t.Cleanup(registry.Reset)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providerName := "prov1"

	err := registry.RegisterProvider(providerName, mockRegisterPluginFactor(false, ctrl))
	Expect(err).NotTo(HaveOccurred())

	err = registry.RegisterProvider(providerName, mockRegisterPluginFactor(false, ctrl))
	Expect(err).To(HaveOccurred())
}

func TestRegistry_GetInstance(t *testing.T) {
	RegisterTestingT(t)
	t.Cleanup(registry.Reset)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
	t.Cleanup(registry.Reset)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	providerName := "prov1"

	p, err := registry.GetPluginInstance(context.TODO(), providerName, &provider.Runtime{})
	Expect(err).To(HaveOccurred())
	Expect(p).To(BeNil())
}

func TestRegistry_ListAllEmpty(t *testing.T) {
	RegisterTestingT(t)
	t.Cleanup(registry.Reset)

	registered := registry.ListPlugins()
	Expect(registered).To(HaveLen(0))
}

func TestRegistry_ListAllNotEmpty(t *testing.T) {
	RegisterTestingT(t)
	t.Cleanup(registry.Reset)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	err := registry.RegisterProvider("prov1", mockRegisterPluginFactor(false, ctrl))
	Expect(err).NotTo(HaveOccurred())

	err = registry.RegisterProvider("prov2", mockRegisterPluginFactor(false, ctrl))
	Expect(err).NotTo(HaveOccurred())

	registered := registry.ListPlugins()
	Expect(registered).To(HaveLen(2))
}

func mockRegisterPluginFactor(shouldFailCreate bool, ctrl *gomock.Controller) provider.Factory {
	return func(ctx context.Context, runtime *provider.Runtime) (provider.MicrovmProvider, error) {
		if shouldFailCreate {
			return nil, fmt.Errorf("some error occured")
		}

		return mock_provider.NewMockMicrovmProvider(ctrl), nil
	}
}
