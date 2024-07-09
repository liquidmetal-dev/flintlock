package models

import (
	"encoding"
	"fmt"
	"strings"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const (
	numPartsForID = 3
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
	uid       string
}

// NewVMID creates a new VMID from a name, namespace and, UID.
func NewVMID(name, namespace, uid string) (*VMID, error) {
	if name == "" {
		return nil, coreerrs.ErrNameRequired
	}

	if namespace == "" {
		namespace = defaults.Namespace
	}

	return &VMID{
		name:      name,
		namespace: namespace,
		uid:       uid,
	}, nil
}

// NewVMIDForce creates a new VMID from a name, namespace, and UID, but without
// any checks. In case we want to create a new UID, but ignore checks.
func NewVMIDForce(name, namespace, uid string) *VMID {
	return &VMID{
		name:      name,
		namespace: namespace,
		uid:       uid,
	}
}

// NewVMID creates a new VMID from a string.
func NewVMIDFromString(id string) (*VMID, error) {
	ns, name, uid, err := splitVMIDFromString(id)
	if err != nil {
		return nil, fmt.Errorf("populating id from string: %w", err)
	}

	return NewVMID(name, ns, uid)
}

// Name returns the name part of the VMID.
func (v *VMID) Name() string {
	return v.name
}

// Namespace returns the namespace part of the VMID.
func (v *VMID) Namespace() string {
	return v.namespace
}

// UID returns the UID part of the VMID.
func (v *VMID) UID() string {
	return v.uid
}

// String returns a string representation of the vmid.
func (v VMID) String() string {
	return fmt.Sprintf("%s/%s/%s", v.namespace, v.name, v.uid)
}

// MarshalText will marshall the vmid to a string representation.
func (v *VMID) MarshalText() (text []byte, err error) {
	return []byte(v.String()), nil
}

// UnmarshalText will unmarshall the text into the vmid.
func (v *VMID) UnmarshalText(text []byte) error {
	id := string(text)

	ns, name, uid, err := splitVMIDFromString(id)
	if err != nil {
		return fmt.Errorf("parsing vmid from string: %w", err)
	}

	v.name = name
	v.namespace = ns
	v.uid = uid

	return nil
}

// IsEmpty indicates that the id contains blank values.
func (v *VMID) IsEmpty() bool {
	return v.name == "" && v.namespace == ""
}

func (v *VMID) SetUID(uid string) {
	v.uid = uid
}

func splitVMIDFromString(id string) (namespace, name, uid string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != numPartsForID {
		return "", "", "", coreerrs.IncorrectVMIDFormatError{ActualID: id}
	}

	if parts[0] == "" {
		return "", "", "", coreerrs.ErrNamespaceRequired
	}

	if parts[1] == "" {
		return "", "", "", coreerrs.ErrNameRequired
	}

	if parts[2] == "" {
		return "", "", "", coreerrs.ErrUIDRequired
	}

	return parts[0], parts[1], parts[2], nil
}
