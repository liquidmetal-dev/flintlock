package application_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/instance"
	"github.com/liquidmetal-dev/flintlock/core/application"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
)

const (
	testUID = "34b8fc8e-6246-11ec-90d6-0242ac120003"
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
			specToCreate: createTestSpec("", "", ""),
			expectError:  false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				im.GenerateRandom().Return("id1234", nil).Times(1)

				pm.Capabilities().Return(models.Capabilities{models.MetadataServiceCapability})

				im.GenerateRandom().Return(testUID, nil).Times(1)

				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						Name:      "id1234",
						Namespace: defaults.MicroVMNamespace,
						UID:       testUID,
					}),
				).Return(nil, nil)

				expectedCreatedSpec := createTestSpecWithMetadata("id1234", defaults.MicroVMNamespace, testUID, createInstanceMetadatadata(t, testUID))
				expectedCreatedSpec.Spec.Provider = "mock"
				expectedCreatedSpec.Spec.CreatedAt = frozenTime().Unix()
				expectedCreatedSpec.Status.State = models.PendingState

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedCreatedSpec),
				).Return(
					createTestSpec("id1234", defaults.MicroVMNamespace, testUID),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecCreated{
						ID:        "id1234",
						Namespace: defaults.MicroVMNamespace,
						UID:       testUID,
					}),
				)
			},
		},
		{
			name:         "spec with id or namespace, create",
			specToCreate: createTestSpec("id1234", "default", testUID),
			expectError:  false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				pm.Capabilities().Return(models.Capabilities{models.MetadataServiceCapability})
				im.GenerateRandom().Return(testUID, nil).Times(1)
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						Name:      "id1234",
						Namespace: "default",
						UID:       testUID,
					}),
				).Return(
					nil,
					nil,
				)

				expectedCreatedSpec := createTestSpecWithMetadata("id1234", "default", testUID, createInstanceMetadatadata(t, testUID))
				expectedCreatedSpec.Spec.Provider = "mock"
				expectedCreatedSpec.Spec.CreatedAt = frozenTime().Unix()
				expectedCreatedSpec.Status.State = models.PendingState

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedCreatedSpec),
				).Return(
					createTestSpec("id1234", "default", testUID),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecCreated{
						ID:        "id1234",
						Namespace: "default",
						UID:       testUID,
					}),
				)
			},
		},
		{
			name:         "spec already exists, should fail",
			specToCreate: createTestSpec("id1234", "default", testUID),
			expectError:  true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				im.GenerateRandom().Return(testUID, nil).Times(1)
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						Name:      "id1234",
						Namespace: "default",
						UID:       testUID,
					}),
				).Return(
					createTestSpec("id1234", "default", testUID),
					nil,
				)
			},
		},
		{
			name:         "spec with id, namespace and existing instance data create",
			specToCreate: createTestSpecWithMetadata("id1234", "default", testUID, createInstanceMetadatadata(t, "abcdef")),
			expectError:  false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				pm.Capabilities().Return(models.Capabilities{models.MetadataServiceCapability})
				im.GenerateRandom().Return(testUID, nil).Times(1)
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						Name:      "id1234",
						Namespace: "default",
						UID:       testUID,
					}),
				).Return(
					nil,
					nil,
				)

				expectedCreatedSpec := createTestSpecWithMetadata("id1234", "default", testUID, createInstanceMetadatadata(t, "abcdef"))
				expectedCreatedSpec.Spec.Provider = "mock"
				expectedCreatedSpec.Spec.CreatedAt = frozenTime().Unix()
				expectedCreatedSpec.Status.State = models.PendingState

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedCreatedSpec),
				).Return(
					createTestSpecWithMetadata("id1234", "default", testUID, createInstanceMetadatadata(t, "abcdef")),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecCreated{
						ID:        "id1234",
						Namespace: "default",
						UID:       testUID,
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
				Repo: rm,
				MicrovmProviders: map[string]ports.MicroVMService{
					"mock": pm,
				},
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{DefaultProvider: "mock"}, ports)
			_, err := app.CreateMicroVM(ctx, tc.specToCreate)

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
		toDeleteUID string
		expectError bool
		expect      func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder)
	}{
		{
			name:        "empty id, should fail",
			expectError: true,
			toDeleteUID: "",
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
			},
		},
		{
			name:        "spec exists, should delete",
			expectError: false,
			toDeleteUID: testUID,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						UID: testUID,
					}),
				).Return(
					createTestSpec("id1234", "default", testUID),
					nil,
				)

				expectedUpdatedSpec := createTestSpec("id1234", "default", testUID)
				expectedUpdatedSpec.Spec.DeletedAt = frozenTime().Unix()
				expectedUpdatedSpec.Status.State = models.DeletingState

				rm.Save(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(expectedUpdatedSpec),
				).Return(
					createTestSpec("id1234", "default", testUID),
					nil,
				)

				em.Publish(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(defaults.TopicMicroVMEvents),
					gomock.Eq(&events.MicroVMSpecUpdated{
						ID:        "id1234",
						Namespace: "default",
						UID:       testUID,
					}),
				)
			},
		},
		{
			name:        "spec doesn't exist, should not delete",
			expectError: true,
			toDeleteUID: testUID,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						UID: testUID,
					}),
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
				Repo: rm,
				MicrovmProviders: map[string]ports.MicroVMService{
					"mock": pm,
				},
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{DefaultProvider: "mock"}, ports)
			err := app.DeleteMicroVM(ctx, tc.toDeleteUID)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestApp_GetMicroVM(t *testing.T) {
	frozenTime := time.Now

	tt := []struct {
		name        string
		toGetUID    string
		expectError bool
		expect      func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder)
	}{
		{
			name:        "empty id should return an error",
			toGetUID:    "",
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
			},
		},
		{
			name:        "spec not found should return an error",
			toGetUID:    testUID,
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						UID: testUID,
					}),
				).Return(
					nil,
					nil,
				)
			},
		},
		{
			name:        "should return an error when rm.Get returns an error",
			toGetUID:    testUID,
			expectError: true,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						UID: testUID,
					}),
				).Return(
					nil,
					errors.New("an random error occurred"),
				)
			},
		},
		{
			name:        "microvm with id exists in namespace and is returned",
			toGetUID:    testUID,
			expectError: false,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.Get(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(ports.RepositoryGetOptions{
						UID: testUID,
					}),
				).Return(
					createTestSpec("id1234", "default", testUID),
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
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
				Repo: rm,
				MicrovmProviders: map[string]ports.MicroVMService{
					"mock": pm,
				},
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{DefaultProvider: "mock"}, ports)
			mvm, err := app.GetMicroVM(ctx, tc.toGetUID)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(mvm.Spec).NotTo(BeNil())
			}
		})
	}
}

func TestApp_GetAllMicroVM(t *testing.T) {
	frozenTime := time.Now

	tt := []struct {
		name        string
		toGetNS     string
		toGetName   *string
		expectError bool
		expectedLen int
		expect      func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder)
	}{
		{
			name:        "empty namespace should not return an error",
			toGetNS:     "",
			toGetName:   nil,
			expectError: false,
			expectedLen: 0,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.GetAll(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(models.ListMicroVMQuery{"namespace": ""}),
				).Return(
					nil,
					nil,
				)
			},
		},
		{
			name:        "should return an error when rm.GetAll returns an error",
			toGetNS:     "default",
			toGetName:   nil,
			expectError: true,
			expectedLen: 0,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.GetAll(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(models.ListMicroVMQuery{"namespace": "default"}),
				).Return(
					nil,
					errors.New("a random error occurred"),
				)
			},
		},
		{
			name:        "no microvms in namespace should return empty slice",
			toGetNS:     "default",
			toGetName:   nil,
			expectError: false,
			expectedLen: 0,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.GetAll(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(models.ListMicroVMQuery{"namespace": "default"}),
				).Return(
					nil,
					nil,
				)
			},
		},
		{
			name:        "microvms exist in namespace and are returned",
			toGetNS:     "default",
			toGetName:   nil,
			expectError: false,
			expectedLen: 2,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.GetAll(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(models.ListMicroVMQuery{"namespace": "default"}),
				).Return(
					[]*models.MicroVM{
						createTestSpec("id1234", "default", testUID),
						createTestSpec("id1235", "default", testUID),
					},
					nil,
				)
			},
		},
		{
			name:        "microvms exist in namespace with the same name and are returned",
			toGetNS:     "default",
			toGetName:   ptr.String("id1234"),
			expectError: false,
			expectedLen: 2,
			expect: func(rm *mock.MockMicroVMRepositoryMockRecorder, em *mock.MockEventServiceMockRecorder, im *mock.MockIDServiceMockRecorder, pm *mock.MockMicroVMServiceMockRecorder) {
				rm.GetAll(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Eq(models.ListMicroVMQuery{
						"namespace": "default",
						"name":      "id1234",
					}),
				).Return(
					[]*models.MicroVM{
						createTestSpec("id1234", "default", "uid1"),
						createTestSpec("id1234", "default", "uid2"),
					},
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
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
				Repo: rm,
				MicrovmProviders: map[string]ports.MicroVMService{
					"mock": pm,
				},
				EventService:      em,
				IdentifierService: im,
				NetworkService:    ns,
				ImageService:      is,
				FileSystem:        fs,
				Clock:             frozenTime,
			}

			tc.expect(rm.EXPECT(), em.EXPECT(), im.EXPECT(), pm.EXPECT())

			ctx := context.Background()
			app := application.New(&application.Config{DefaultProvider: "mock"}, ports)
			query := models.ListMicroVMQuery{"namespace": tc.toGetNS}

			if tc.toGetName != nil {
				query["name"] = *tc.toGetName
			}

			mvms, err := app.GetAllMicroVM(ctx, query)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(len(mvms)).To(Equal(tc.expectedLen))
				if len(mvms) > 0 {
					for _, mvm := range mvms {
						Expect(mvm.Spec).NotTo(BeNil())
						Expect(mvm.ID.Name()).NotTo(Equal(""))
						Expect(mvm.ID.Namespace()).To(Equal(tc.toGetNS))
					}
				}
			}
		})
	}
}

func createTestSpec(name, ns, uid string) *models.MicroVM {
	return createTestSpecWithMetadata(name, ns, uid, map[string]string{})
}

func createTestSpecWithMetadata(name, ns, uid string, metadata map[string]string) *models.MicroVM {
	var vmid *models.VMID

	if uid == "" {
		uid = testUID
	}

	if name == "" && ns == "" {
		vmid = &models.VMID{}
	} else {
		vmid, _ = models.NewVMID(name, ns, testUID)
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
					Type:                  models.IfaceTypeTap,
				},
				{
					AllowMetadataRequests: false,
					GuestDeviceName:       "eth1",
					Type:                  models.IfaceTypeMacvtap,
				},
			},
			RootVolume: models.Volume{
				ID:         "root",
				IsReadOnly: false,
				Source: models.VolumeSource{
					Container: &models.ContainerVolumeSource{
						Image: "docker.io/library/ubuntu:groovy",
					},
				},
				Size: 20000,
			},
			Metadata:  metadata,
			CreatedAt: 0,
			UpdatedAt: 0,
			DeletedAt: 0,
		},
	}
}

func createInstanceMetadatadata(t *testing.T, instanceID string) map[string]string {
	RegisterTestingT(t)

	instanceData := instance.New(instance.WithInstanceID(instanceID))
	data, err := yaml.Marshal(instanceData)
	Expect(err).NotTo(HaveOccurred())

	instanceDataStr := base64.StdEncoding.EncodeToString(data)

	return map[string]string{
		"meta-data": instanceDataStr,
	}
}
