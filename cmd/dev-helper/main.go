//nolint
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/containerd/containerd/namespaces"
	"github.com/sirupsen/logrus"

	_ "github.com/containerd/containerd/api/events"

	ctr "github.com/containerd/containerd"

	"github.com/weaveworks/reignite/api/events"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/containerd"
	"github.com/weaveworks/reignite/infrastructure/controllers"
	"github.com/weaveworks/reignite/pkg/defaults"
	rlog "github.com/weaveworks/reignite/pkg/log"
)

//NOTE: this is a temporary app to help with development

const (
	vmName      = "vm1"
	vmNamespace = "teamabc"
	imageName   = "docker.io/library/ubuntu:groovy"
)

func main() {
	//socketPath := defaults.ContainerdSocket
	socketPath := "/home/richard/code/scratch/containerdlocal/run/containerd.sock"

	ctx, cancel := context.WithCancel(context.Background())

	rlog.Configure(&rlog.Config{
		Verbosity: 10,
		Format:    "text",
		Output:    "stderr",
	})
	logger := rlog.GetLogger(ctx)
	logger.Infof("reignite dev-helper, using containerd socket: %s", socketPath)

	//eventPublishTest(ctx, socketPath, logger)

	logger.Info("starting containerd event listener")
	go eventListener(ctx, socketPath, logger)

	logger.Infof("Press [enter] to run controller test")
	//fmt.Scanln()
	go controllerTest(ctx, socketPath, logger)
	go publishPeriodicEvents(ctx, socketPath, logger)

	//logger.Infof("Press [enter] to write vmspec to using containerd repo")
	//fmt.Scanln()
	//repoTest(ctx, socketPath, logger)

	//logger.Infof("Press [enter] to get image %s", imageName)
	//fmt.Scanln()
	//imageServiceTest(ctx, socketPath, logger)

	logger.Info("Press [enter] to exit")
	fmt.Scanln()

	cancel()
}

func eventPublishTest(ctx context.Context, socketPath string, logger *logrus.Entry) {
	cfg := &containerd.Config{
		SocketPath: socketPath,
	}
	logger.Info("creating event service")

	es, err := containerd.NewEventService(cfg)
	if err != nil {
		log.Fatal(err)
	}

	evt := &events.MicroVMSpecCreated{
		ID:        "abcdf",
		Namespace: "ns1",
	}

	ctx, cancel := context.WithCancel(ctx)

	evts, errs := es.Subscribe(ctx)

	err = es.Publish(ctx, "/test", evt)
	if err != nil {
		log.Fatal(err)
	}

	select {
	case evt := <-evts:
		fmt.Printf("in dev-helper, got evtenr: %#v\n", evt.Event)
	case evtErr := <-errs:
		fmt.Println(evtErr)
	}

	cancel()
}

func repoTest(ctx context.Context, socketPath string, logger *logrus.Entry) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	repo := containerd.NewMicroVMRepoWithClient(client)

	vmSpec := getTestSpec()
	logger.Infof("saving microvm spec %s", vmSpec.ID)

	_, err = repo.Save(ctx, vmSpec)
	if err != nil {
		log.Fatal(err)
	}
}

func imageServiceTest(ctx context.Context, socketPath string, logger *logrus.Entry) {
	cfg := &containerd.Config{
		//Snapshotter: defaults.ContainerdSnapshotter,
		//Snapshotter: "overlayfs",
		SnapshotterKernel: "native",
		SnapshotterVolume: "native",
		SocketPath:        socketPath,
	}
	logger.Infof("using snapshotters %s & %s", cfg.SnapshotterKernel, cfg.SnapshotterVolume)

	imageService, err := containerd.NewImageService(cfg)
	if err != nil {
		log.Fatal(err)
	}

	input := ports.GetImageInput{
		ImageName:      imageName,
		OwnerName:      vmName,
		OwnerNamespace: vmNamespace,
		Use:            models.ImageUseVolume,
	}
	mountPoint, err := imageService.GetAndMount(ctx, input)
	if err != nil {
		log.Fatal(err)
	}

	logger.Infof("mounted image %s to %s (type %s)", imageName, mountPoint[0].Source, mountPoint[0].Type)
}

func publishPeriodicEvents(ctx context.Context, socketPath string, logger *logrus.Entry) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	tickCh := time.Tick(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Cancelled, exiting publisher")
			return
		case <-tickCh:
			logger.Info("publishing event")
			createEvt := &events.MicroVMSpecCreated{
				ID:        "vm1",
				Namespace: "testns",
			}

			err = client.EventService().Publish(namespaceCtx, defaults.TopicMicroVMEvents, createEvt)
			if err != nil {
				logger.Error(err)
			}

		}
	}
}

func controllerTest(ctx context.Context, socketPath string, logger *logrus.Entry) {
	em, err := containerd.NewEventService(&containerd.Config{
		SocketPath: socketPath,
	})
	if err != nil {
		log.Fatal(err)
	}

	app := &testApp{
		logger: logger,
	}

	controller := controllers.New(em, app)

	logger.Info("running controller")
	controller.Run(ctx, 1)
	logger.Info("controller not running")

}

func eventListener(ctx context.Context, socketPath string, logger *logrus.Entry) {
	cfg := &containerd.Config{
		SocketPath: socketPath,
	}
	logger.Info("creating event service")

	es, err := containerd.NewEventService(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ch, errsCh := es.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.Info("Existing event listener")
			return
		case evt := <-ch:
			logger.Infof("event received, ns %s, topic %s, body: %#v", evt.Namespace, evt.Topic, evt.Event)
		case errEvt := <-errsCh:
			logger.Errorf("event error received: %s", errEvt)
		}
	}
}

func getTestSpec() *models.MicroVM {
	vmid, _ := models.NewVMID(vmName, vmNamespace)
	return &models.MicroVM{
		ID: *vmid,
		Spec: models.MicroVMSpec{
			MemoryInMb: 2048,
			VCPU:       4,
			Kernel: models.Kernel{
				Image:   "docker.io/linuxkit/kernel:5.4.129",
				CmdLine: "console=ttyS0 reboot=k panic=1 pci=off i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd ds=nocloud-net;s=http://169.254.169.254/latest/ network-config=ASDFGFDFG",
			},
			NetworkInterfaces: []models.NetworkInterface{
				{
					AllowMetadataRequests: false,
					GuestMAC:              "AA:FF:00:00:00:01",
					HostDeviceName:        "tap1",
					GuestDeviceName:       "eth0",
				},
				{
					AllowMetadataRequests: false,
					HostDeviceName:        "/dev/tap55",
					GuestDeviceName:       "eth1",
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
							Image: imageName,
						},
					},
					Size: 20000,
				},
			},
		},
	}
}

type testApp struct {
	logger *logrus.Entry
}

func (t *testApp) ReconcileMicroVM(ctx context.Context, id, namespace string) error {
	t.logger.Infof("received request to reconcile %s/%s", id, namespace)

	return nil
}
