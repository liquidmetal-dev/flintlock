package containerd

import (
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const (
	// MicroVMSpecType is the type name for a microvm spec.
	MicroVMSpecType = "microvm"

	nameLabelFormat      = "%s/name"
	namespaceLabelFormat = "%s/ns"
	typeLabelFormat      = "%s/type"
	versionLabelFormat   = "%s/version"
	uidLabelFormat       = "%s/uid"
)

func contentRefName(microvm *models.MicroVM) string {
	return fmt.Sprintf("%s/microvm/%s", defaults.Domain, microvm.ID.String())
}

func labelFilter(name, value string) string {
	return fmt.Sprintf("labels.\"%s\"==\"%s\"", name, value)
}

// NameLabel is the name of the containerd content store label used for the microvm name.
func NameLabel() string {
	return fmt.Sprintf(nameLabelFormat, defaults.Domain)
}

// NamespaceLabel is the name of the containerd content store label used for the microvm namespace.
func NamespaceLabel() string {
	return fmt.Sprintf(namespaceLabelFormat, defaults.Domain)
}

// TypeLabel is the name of the containerd content store label used to denote the type of content.
func TypeLabel() string {
	return fmt.Sprintf(typeLabelFormat, defaults.Domain)
}

// VersionLabel is the name of the containerd content store label to hold version of the content.
func VersionLabel() string {
	return fmt.Sprintf(versionLabelFormat, defaults.Domain)
}

// UIDLabel is the name of the containerd content store label to hold UID of the content.
func UIDLabel() string {
	return fmt.Sprintf(uidLabelFormat, defaults.Domain)
}
