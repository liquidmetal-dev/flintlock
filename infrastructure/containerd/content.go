package containerd

import (
	"fmt"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/defaults"
)

var (
	// IDLabel is the name of the containerd content store label used for the microvm identifier.
	IDLabel = fmt.Sprintf("%s/vmid", defaults.Domain)
	// NamespaceLabel is the name of the containerd content store label used for the microvm namespace.
	NamespaceLabel = fmt.Sprintf("%s/ns", defaults.Domain)
	// TypeLabel is the name of the containerd content store label used to denote the type of content.
	TypeLabel = fmt.Sprintf("%s/type", defaults.Domain)
	// VersionLabel is the name of the containerd content store label to hold version of the content.
	VersionLabel = fmt.Sprintf("%s/version", defaults.Domain)
)

func contentRefName(microvm *models.MicroVM) string {
	return fmt.Sprintf("%s/%s", microvm.Namespace, microvm.ID)
}

func labelFilter(name, value string) string {
	return fmt.Sprintf("labels.\"%s\"==\"%s\"", name, value)
}
