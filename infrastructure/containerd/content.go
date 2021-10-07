package containerd

import (
	"fmt"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/defaults"
)

var (
	// NameLabel is the name of the containerd content store label used for the microvm name.
	NameLabel = fmt.Sprintf("%s/name", defaults.Domain)
	// NamespaceLabel is the name of the containerd content store label used for the microvm namespace.
	NamespaceLabel = fmt.Sprintf("%s/ns", defaults.Domain)
	// TypeLabel is the name of the containerd content store label used to denote the type of content.
	TypeLabel = fmt.Sprintf("%s/type", defaults.Domain)
	// VersionLabel is the name of the containerd content store label to hold version of the content.
	VersionLabel = fmt.Sprintf("%s/version", defaults.Domain)
	// MicroVMSpecType is the type name for a microvm spec.
	MicroVMSpecType = "microvm"
)

func contentRefName(microvm *models.MicroVM) string {
	return fmt.Sprintf("%s/microvm/%s", defaults.Domain, microvm.ID)
}

func labelFilter(name, value string) string {
	return fmt.Sprintf("labels.\"%s\"==\"%s\"", name, value)
}
