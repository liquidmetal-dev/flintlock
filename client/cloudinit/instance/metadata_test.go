package instance_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit/instance"
)

const (
	testCloudName    = "nocloud"
	testClusterName  = "cluster1"
	testInstanceID   = "i-123456"
	testHostName     = "host1"
	testPlatformName = "liquidmetal"
)

func TestInstanceMetadata_NewNoOptions(t *testing.T) {
	RegisterTestingT(t)

	m := instance.New()
	Expect(m).NotTo(BeNil())
	Expect(m).To(HaveLen(0))
}

func TestInstanceMetadata_NewWithOptions(t *testing.T) {
	RegisterTestingT(t)

	m := instance.New(
		instance.WithCloudName(testCloudName),
		instance.WithClusterName(testClusterName),
		instance.WithInstanceID(testInstanceID),
		instance.WithLocalHostname(testHostName),
		instance.WithPlatform(testPlatformName),
	)
	Expect(m).NotTo(BeNil())
	Expect(m).To(HaveLen(5))

	Expect(m[instance.CloudNameKey]).To(Equal(testCloudName))
	Expect(m[instance.ClusterNameKey]).To(Equal(testClusterName))
	Expect(m[instance.InstanceIDKey]).To(Equal(testInstanceID))
	Expect(m[instance.LocalHostnameKey]).To(Equal(testHostName))
	Expect(m[instance.PlatformKey]).To(Equal(testPlatformName))
}

func TestInstanceMetadata_NewOptionsExisting(t *testing.T) {
	RegisterTestingT(t)

	existing := instance.New(
		instance.WithCloudName(testCloudName),
		instance.WithClusterName(testClusterName),
		instance.WithInstanceID(testInstanceID),
		instance.WithLocalHostname(testHostName),
		instance.WithPlatform(testPlatformName),
	)

	m := instance.New(
		instance.WithExisting(existing),
		instance.WithInstanceID("changed"),
	)

	Expect(m).NotTo(BeNil())
	Expect(m).To(HaveLen(5))

	Expect(m[instance.CloudNameKey]).To(Equal(testCloudName))
	Expect(m[instance.ClusterNameKey]).To(Equal(testClusterName))
	Expect(m[instance.InstanceIDKey]).To(Equal("changed"))
	Expect(m[instance.LocalHostnameKey]).To(Equal(testHostName))
	Expect(m[instance.PlatformKey]).To(Equal(testPlatformName))
}
