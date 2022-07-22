package metadata

import "github.com/weaveworks-liquidmetal/flintlock/client/cloudinit"

// CloudInitFilter will filter metadata items that are for cloud-init.
func CloudInitFilter(keyName, _ string) bool {
	switch keyName {
	case cloudinit.InstanceDataKey:
		return true
	case cloudinit.NetworkConfigDataKey:
		return true
	case cloudinit.UserdataKey:
		return true
	case cloudinit.VendorDataKey:
		return true
	default:
		return false
	}
}

// NotCloudInitFilter will filter metadata items that are not for cloud-init.
func NotCloudInitFilter(keyName, _ string) bool {
	return !CloudInitFilter(keyName, "")
}
