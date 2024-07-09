package models_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

const (
	testUID = "344780b0-6249-11ec-90d6-0242ac120003"
)

func TestVMID_New(t *testing.T) {
	RegisterTestingT(t)

	testCases := []struct {
		name        string
		vmName      string
		vmNamespace string
		expectError bool
	}{
		{
			name:        "non empty name and namespace, should succeed",
			vmName:      "test1",
			vmNamespace: "ns1",
			expectError: false,
		},
		{
			name:        "empty name, non-empty namespace, should error",
			vmName:      "",
			vmNamespace: "ns1",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vmid, err := models.NewVMID(tc.vmName, tc.vmNamespace, testUID)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(vmid).NotTo(BeNil())
			}
		})
	}
}

func TestVMID_NewFromString(t *testing.T) {
	RegisterTestingT(t)

	testCases := []struct {
		name         string
		input        string
		expectedNS   string
		expectedName string
		expectError  bool
	}{
		{
			name:         "correct structure",
			input:        "ns1/test1/344780b0-6249-11ec-90d6-0242ac120003",
			expectError:  false,
			expectedNS:   "ns1",
			expectedName: "test1",
		},
		{
			name:        "empty ns",
			input:       "/test1",
			expectError: true,
		},
		{
			name:        "empty name",
			input:       "ns1/",
			expectError: true,
		},
		{
			name:        "wrong format",
			input:       "ns1@test1",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vmid, err := models.NewVMIDFromString(tc.input)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(vmid).NotTo(BeNil())
				Expect(vmid.Name()).To(Equal(tc.expectedName))
				Expect(vmid.Namespace()).To(Equal(tc.expectedNS))
			}
		})
	}
}

func TestVMID_Marshalling(t *testing.T) {
	RegisterTestingT(t)

	id1, err := models.NewVMID("name", "namespace", testUID)
	Expect(err).ToNot(HaveOccurred())

	data, err := json.Marshal(id1)
	Expect(err).NotTo(HaveOccurred())

	id2 := &models.VMID{}

	Expect(json.Unmarshal(data, id2)).NotTo(HaveOccurred())
	Expect(id2).To(BeEquivalentTo(id1))
}

func TestVMID_EmbeddedMarshalling(t *testing.T) {
	RegisterTestingT(t)

	id1, err := models.NewVMID("name", "namespace", testUID)
	Expect(err).ToNot(HaveOccurred())

	s1 := struct {
		ID *models.VMID `json:"id"`
	}{
		ID: id1,
	}

	data, err := json.Marshal(s1)
	Expect(err).NotTo(HaveOccurred())

	s2 := struct {
		ID *models.VMID `json:"id"`
	}{}

	err = json.Unmarshal(data, &s2)
	Expect(err).NotTo(HaveOccurred())
	Expect(s2).To(BeEquivalentTo(s1))
}
