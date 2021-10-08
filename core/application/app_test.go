package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/weaveworks/reignite/api/events"
	"github.com/weaveworks/reignite/core/application"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/mock"
	"github.com/weaveworks/reignite/pkg/defaults"
)

func TestApp_CreateMicroVM(t *testing.T) {
	frozenTime := time.Now

	testCases := []struct {
		name         string
		specToCreate *models.MicroVM
		expectError  bool
		expect       func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder)
	}{
		{
			name:        "nil spec, should fail",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
			},
		},
		{
			name:         "spec with no id or namespace, create id/ns and create",
			specToCreate: createTestSpec("", ""),
			expectError:  false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				im.GenerateRandom().Return("id1234", nil)

				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq(defaults.MicroVMNamespace),
				).Return(
					nil,
					nil,
				)

				expectedCreatedSpec := createTestSpec("id1234", defaults.MicroVMNamespace)
				expectedCreatedSpec.Spec.CreatedAt = frozenTime().Unix()

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedCreatedSpec),
				).Return(
					createTestSpec("id1234", defaults.MicroVMNamespace),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecCreated{
						ID:        "id1234",
						Namespace: defaults.MicroVMNamespace,
					}),
				)
			},
		},
		{
			name:         "spec with id or namespace, create",
			specToCreate: createTestSpec("id1234", "default"),
			expectError:  false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					nil,
					nil,
				)

				expectedCreatedSpec := createTestSpec("id1234", "default")
				expectedCreatedSpec.Spec.CreatedAt = frozenTime().Unix()

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedCreatedSpec),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecCreated{
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
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
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

			rm := mock.NewMockMicroVMRepository(mockCtrl)
			em := mock.NewMockEventService(mockCtrl)
			im := mock.NewMockIDService(mockCtrl)
			pm := mock.NewMockMicroVMService(mockCtrl)
			ns := mock.NewMockNetworkService(mockCtrl)
			is := mock.NewMockImageService(mockCtrl)
			fs := afero.NewMemMapFs()
			ports := &ports.Collection{
				Repo:              rm,
				Provider:          pm,
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{}, ports)
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
	frozenTime := time.Now

	testCases := []struct {
		name         string
		specToUpdate *models.MicroVM
		expectError  bool
		expect       func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder)
	}{
		{
			name:        "nil spec, should fail",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
			},
		},
		{
			name:         "spec with no id or namespace, should fail",
			specToUpdate: createTestSpec("", ""),
			expectError:  true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
			},
		},
		{
			name:         "spec is valid and update is valid, update",
			specToUpdate: createTestSpec("id1234", "default"),
			expectError:  false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				expectedUpdatedSpec := createTestSpec("id1234", "default")
				expectedUpdatedSpec.Spec.UpdatedAt = frozenTime().Unix()

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedUpdatedSpec),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecUpdated{
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

			rm := mock.NewMockMicroVMRepository(mockCtrl)
			em := mock.NewMockEventService(mockCtrl)
			im := mock.NewMockIDService(mockCtrl)
			pm := mock.NewMockMicroVMService(mockCtrl)
			ns := mock.NewMockNetworkService(mockCtrl)
			is := mock.NewMockImageService(mockCtrl)
			fs := afero.NewMemMapFs()
			ports := &ports.Collection{
				Repo:              rm,
				Provider:          pm,
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{}, ports)
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
	frozenTime := time.Now

	testCases := []struct {
		name        string
		toDeleteID  string
		toDeleteNS  string
		expectError bool
		expect      func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder)
	}{
		{
			name:        "empty id, should fail",
			expectError: true,
			toDeleteID:  "",
			toDeleteNS:  "default",
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
			},
		},
		{
			name:        "spec exists, should delete",
			expectError: false,
			toDeleteID:  "id1234",
			toDeleteNS:  "default",
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq("id1234"),
					gomock.Eq("default"),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				expectedUpdatedSpec := createTestSpec("id1234", "default")
				expectedUpdatedSpec.Spec.DeletedAt = frozenTime().Unix()

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedUpdatedSpec),
				).Return(
					createTestSpec("id1234", "default"),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecUpdated{
						ID:        "id1234",
						Namespace: "default",
					}),
				)
			},
		},
		{
			name:        "spec doesn't exist, should not delete",
			expectError: true,
			toDeleteID:  "id1234",
			toDeleteNS:  "default",
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
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

			rm := mock.NewMockMicroVMRepository(mockCtrl)
			em := mock.NewMockEventService(mockCtrl)
			im := mock.NewMockIDService(mockCtrl)
			pm := mock.NewMockMicroVMService(mockCtrl)
			ns := mock.NewMockNetworkService(mockCtrl)
			is := mock.NewMockImageService(mockCtrl)
			fs := afero.NewMemMapFs()
			ports := &ports.Collection{
				Repo:              rm,
				Provider:          pm,
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{}, ports)
			err := app.DeleteMicroVM(ctx, tc.toDeleteID, tc.toDeleteNS)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createTestSpec(name, ns string) *models.MicroVM {
	var vmid *models.VMID

	if name == "" && ns == "" {
		vmid = &models.VMID{}
	} else {
		vmid, _ = models.NewVMID(name, ns)
	}

	return &models.MicroVM{
		ID: *vmid,
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
			CreatedAt: 0,
			UpdatedAt: 0,
			DeletedAt: 0,
		},
	}
}
