//nolint
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/sirupsen/logrus"

	_ "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"

	ctr "github.com/containerd/containerd"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/containerd"
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
		Verbosity: 0,
		Format:    "text",
		Output:    "stderr",
	})
	logger := rlog.GetLogger(ctx)
	logger.Infof("reignite dev-helper, using containerd socket: %s", socketPath)

	logger.Info("starting containerd event listener")
	go eventListener(ctx, socketPath, logger)

	logger.Infof("Press [enter] to write vmspec to using containerd repo")
	fmt.Scanln()
	repoTest(ctx, socketPath, logger)

	logger.Infof("Press [enter] to get image %s", imageName)
	fmt.Scanln()
	imageServiceTest(ctx, socketPath, logger)

	//repoUpdateTest(ctx, socketPath)
	//imageLeaseTest(ctx, socketPath)
	//contentStoreTest(ctx, socketPath)

	logger.Info("Press [enter] to exit")
	fmt.Scanln()

	cancel()
}

func repoTest(ctx context.Context, socketPath string, logger *logrus.Entry) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	repo := containerd.NewMicroVMRepoWithClient(client)

	vmSpec := getTestSpec()
	logger.Infof("saving microvm spec %s/%s", vmSpec.Namespace, vmSpec.ID)

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

func eventListener(ctx context.Context, socketPath string, logger *logrus.Entry) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	es := client.EventService()
	ch, errsCh := es.Subscribe(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Existing event listener")
		case evt := <-ch:
			v, err := typeurl.UnmarshalAny(evt.Event)
			if err != nil {
				logger.Errorf("error unmarshalling: %s", err)
				continue
			}
			out, err := json.Marshal(v)
			if err != nil {
				logger.Errorf("cannot marshal Any into JSON: %s", err)
				continue
			}
			logger.Infof("event received, ns %s, topic %s, body: %s", evt.Namespace, evt.Topic, string(out))
		case errEvt := <-errsCh:
			logger.Errorf("event error received: %s", errEvt)
		}
	}
}

func imageLeaseTest(ctx context.Context, socketPath string) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	nsCtx := namespaces.WithNamespace(ctx, vmNamespace)

	leaseManager := client.LeasesService()
	l, err := leaseManager.Create(nsCtx, leases.WithID("mytestlease"))
	if err != nil {
		log.Fatal(err)
	}

	leaseCtx := leases.WithLease(nsCtx, l.ID)

	image, err := client.Pull(leaseCtx, imageName, ctr.WithPullUnpack)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", image)
	fmt.Println("done with pull")
}

func contentStoreTest(ctx context.Context, socketPath string) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	nsCtx := namespaces.WithNamespace(ctx, vmNamespace)

	leaseManager := client.LeasesService()
	l, err := leaseManager.Create(nsCtx, leases.WithID("mytestlease"))
	if err != nil {
		log.Fatal(err)
	}

	vmSpec := getTestSpec()

	leaseCtx := leases.WithLease(nsCtx, l.ID)

	store := client.ContentStore()

	refName := "mytestrefname"
	writer, err := store.Writer(leaseCtx, content.WithRef(refName))
	if err != nil {
		log.Fatal(err)
	}

	data, err := json.Marshal(vmSpec)
	if err != nil {
		log.Fatal(err)
	}

	_, err = writer.Write(data)
	if err != nil {
		log.Fatal(err)
	}

	labels := map[string]string{
		"vmid": vmName,
		"ns":   vmNamespace,
	}
	err = writer.Commit(leaseCtx, 0, "", content.WithLabels(labels))
	if err != nil {
		log.Fatal(err)
	}

	writer.Close()
}

func repoUpdateTest(ctx context.Context, socketPath string) {
	client, err := ctr.New(socketPath)
	if err != nil {
		log.Fatal(err)
	}

	repo := containerd.NewMicroVMRepoWithClient(client)

	vmSpec := getTestSpec()

	_, err = repo.Save(ctx, vmSpec)
	if err != nil {
		log.Fatal(err)
	}

	vmSpec.Spec.MemoryInMb = 8096

	_, err = repo.Save(ctx, vmSpec)
	if err != nil {
		log.Fatal(err)
	}

	specs, err := repo.GetAll(ctx, vmNamespace)
	if err != nil {
		log.Fatal(err)
	}

	for _, spec := range specs {
		log.Printf("spec: %#v\n", spec)
	}

}

func getTestSpec() *models.MicroVM {
	return &models.MicroVM{
		ID:        vmName,
		Namespace: vmNamespace,
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
