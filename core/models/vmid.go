package models

import (
	"encoding"
	"fmt"
	"strings"

	coreerrs "github.com/weaveworks/flintlock/core/errors"
)

const (
	numPartsForID = 2
)

var (
	_ encoding.TextMarshaler   = (*VMID)(nil)
	_ encoding.TextUnmarshaler = (*VMID)(nil)
	_ fmt.Stringer             = (*VMID)(nil)
)

// VMID represents the identifier for a microvm.
type VMID struct {
	name      string
	namespace string
}

// NewVMID creates a new VMID from a name and namespace.
func NewVMID(name, namespace string) (*VMID, error) {
	if name == "" {
		return nil, coreerrs.ErrNameRequired
	}
	if namespace == "" {
		return nil, coreerrs.ErrNamespaceRequired
	}

	return &VMID{
		name:      name,
		namespace: namespace,
	}, nil
}

// NewVMID creates a new VMID from a string.
func NewVMIDFromString(id string) (*VMID, error) {
	ns, name, err := splitVMIDFromString(id)
	if err != nil {
		return nil, fmt.Errorf("populating id from string: %w", err)
	}

	return NewVMID(name, ns)
}

// Name returns the name part of the VMID.
func (v *VMID) Name() string {
	return v.name
}

// Namespace returns the namespace part of the VMID.
func (v *VMID) Namespace() string {
	return v.namespace
}

// String returns a string representation of the vmid.
func (v VMID) String() string {
	return fmt.Sprintf("%s/%s", v.namespace, v.name)
}

// MarshalText will marshall the vmid to a string representation.
func (v *VMID) MarshalText() (text []byte, err error) {
	return []byte(v.String()), nil
}

// UnmarshalText will unmarshall the text into the vmid.
func (v *VMID) UnmarshalText(text []byte) error {
	id := string(text)

	ns, name, err := splitVMIDFromString(id)
	if err != nil {
		return fmt.Errorf("parsing vmid from string: %w", err)
	}

	v.name = name
	v.namespace = ns

	return nil
}

// IsEmpty indicates that the id contains blank values.
func (v *VMID) IsEmpty() bool {
	return v.name == "" && v.namespace == ""
}

func splitVMIDFromString(id string) (namespace string, name string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != numPartsForID {
		return "", "", coreerrs.IncorrectVMIDFormatError{ActualID: id}
	}

	if parts[0] == "" {
		return "", "", coreerrs.ErrNamespaceRequired
	}
	if parts[1] == "" {
		return "", "", coreerrs.ErrNameRequired
	}

	return parts[0], parts[1], nil
}
