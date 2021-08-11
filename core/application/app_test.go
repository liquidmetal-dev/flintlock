package application_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/weaveworks/reignite/core/application"
	"github.com/weaveworks/reignite/core/events"
	"github.com/weaveworks/reignite/core/models"
	prvmock "github.com/weaveworks/reignite/infrastructure/providers/microvm/mock"
	repomock "github.com/weaveworks/reignite/infrastructure/repositories/microvm/mock"
	eventmock "github.com/weaveworks/reignite/infrastructure/services/event/mock"
	idmock "github.com/weaveworks/reignite/infrastructure/services/id/mock"
	"github.com/weaveworks/reignite/pkg/defaults"
)

func TestApp_CreateMicroVM(t *testing.T) {
	testCases := []struct {
		name         string
		specToCreate *models.MicroVM
		expectError  bool
		expect       func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder)
	}{
		{
			name:        "nil spec, should fail",
			expectError: true,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
			},
		},
		{
			name:         "spec with no id or namespace, create id/ns and create",
			specToCreate: createTestSpec("", ""),
			expectError:  false,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				im.GenerateRandom().Return("id1234", nil)

				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq(defaults.ContainerdNamespace),
				).Return(
					nil,
					nil,
				)

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(createTestSpec("id1234", defaults.ContainerdNamespace)),
				).Return(
					createTestSpec("id1234", defaults.ContainerdNamespace),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMCreated{
						ID:        "id1234",
						Namespace: defaults.ContainerdNamespace,
					}),
				)
			},
		},
		{
			name:         "spec with id or namespace, create",
			specToCreate: createTestSpec("id1234", "default"),
			expectError:  false,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					nil,
					nil,
				)

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(createTestSpec("id1234", "default")),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMCreated{
						ID:        "id1234",
						Namespace: "default",
					}),
				)
			},
		},
		{
			name:         "spec already exists, should fail",
			specToCreate: createTestSpec("id1234", "default"),
			expectError:  true,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			rm := repomock.NewMockMicroVMRepository(mockCtrl)
			em := eventmock.NewMockEventService(mockCtrl)
			im := idmock.NewMockIDService(mockCtrl)
			pm := prvmock.NewMockMicroVMProvider(mockCtrl)

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(rm, em, im, pm)
			_, err := app.CreateMicroVM(ctx, tc.specToCreate)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestApp_UpdateMicroVM(t *testing.T) {
	testCases := []struct {
		name         string
		specToUpdate *models.MicroVM
		expectError  bool
		expect       func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder)
	}{
		{
			name:        "nil spec, should fail",
			expectError: true,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
			},
		},
		{
			name:         "spec with no id or namespace, should fail",
			specToUpdate: createTestSpec("", ""),
			expectError:  true,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(""),
					gomock.Eq(""),
				).Return(
					nil,
					nil,
				)
			},
		},
		{
			name:         "spec is valid and update is valid, update",
			specToUpdate: createTestSpec("id1234", "default"),
			expectError:  false,
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(createTestSpec("id1234", "default")),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMUpdated{
						ID:        "id1234",
						Namespace: "default",
					}),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			rm := repomock.NewMockMicroVMRepository(mockCtrl)
			em := eventmock.NewMockEventService(mockCtrl)
			im := idmock.NewMockIDService(mockCtrl)
			pm := prvmock.NewMockMicroVMProvider(mockCtrl)

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(rm, em, im, pm)
			_, err := app.UpdateMicroVM(ctx, tc.specToUpdate)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestApp_DeleteMicroVM(t *testing.T) {
	testCases := []struct {
		name        string
		toDeleteID  string
		toDeleteNS  string
		expectError bool
		expect      func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder)
	}{
		{
			name:        "empty id, should fail",
			expectError: true,
			toDeleteID:  "",
			toDeleteNS:  "default",
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
			},
		},
		{
			name:        "spec exists, should delete",
			expectError: false,
			toDeleteID:  "id1234",
			toDeleteNS:  "default",
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				rm.Delete(
					gomock.AssignableToTypeOf(context.Background()),
					createTestSpec("id1234", "default"),
				).Return(nil)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMDeleted{
						ID:        "id1234",
						Namespace: "default",
					}),
				)
			},
		},
		{
			name:        "spec doesn't exist, should not delete",
			expectError: false,
			toDeleteID:  "id1234",
			toDeleteNS:  "default",
			expect: func(rm *repomock.MockMicroVMRepositoryMockRecorder, em *eventmock.MockEventServiceMockRecorder, im *idmock.MockIDServiceMockRecorder, pm *prvmock.MockMicroVMProviderMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					nil,
					nil,
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			rm := repomock.NewMockMicroVMRepository(mockCtrl)
			em := eventmock.NewMockEventService(mockCtrl)
			im := idmock.NewMockIDService(mockCtrl)
			pm := prvmock.NewMockMicroVMProvider(mockCtrl)

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(rm, em, im, pm)
			err := app.DeleteMicroVM(ctx, tc.toDeleteID, tc.toDeleteNS)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createTestSpec(id, namespace string) *models.MicroVM {
	return &models.MicroVM{
		ID:        id,
		Namespace: namespace,
		Spec: models.MicroVMSpec{
			VCPU:       2,
			MemoryInMb: 2048,
			Kernel: models.Kernel{
				Image:    "docker.io/linuxkit/kernel:5.4.129",
				Filename: "kernel",
			},
			NetworkInterfaces: []models.NetworkInterface{
				{
					AllowMetadataRequests: true,
					GuestMAC:              "AA:FF:00:00:00:01",
					GuestDeviceName:       "eth0",
				},
				{
					AllowMetadataRequests: false,
					GuestDeviceName:       "eth1",
					// TODO:
				},
			},
			Volumes: []models.Volume{
				{
					ID:         "root",
					IsRoot:     true,
					IsReadOnly: false,
					MountPoint: "/",
					Source: models.VolumeSource{
						Container: &models.ContainerVolumeSource{
							Image: "docker.io/library/ubuntu:groovy",
						},
					},
					Size: 20000,
				},
			},
		},
	}
}
