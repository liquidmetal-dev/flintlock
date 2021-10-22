//nolint
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/weaveworks/reignite/pkg/cloudinit"
	"github.com/weaveworks/reignite/pkg/ptr"
	"gopkg.in/yaml.v3"

	"github.com/weaveworks/reignite/internal/config"
	"github.com/weaveworks/reignite/internal/inject"

	"github.com/weaveworks/reignite/core/plans"
	"github.com/weaveworks/reignite/pkg/planner"

	"github.com/sirupsen/logrus"

	_ "github.com/containerd/containerd/api/events"

	"github.com/weaveworks/reignite/core/models"
	portsctx "github.com/weaveworks/reignite/core/ports/context"
	"github.com/weaveworks/reignite/pkg/defaults"
	rlog "github.com/weaveworks/reignite/pkg/log"
)

// NOTE: this is a temporary app to help with development

const (
	namespace   = "ns1"
	numNodes    = 2
	socketPath  = "/home/richard/code/scratch/containerdlocal/run/containerd.sock"
	sshKeyPath  = "/home/richard/.ssh/id_ed25519.pub"
	rootfsImage = "docker.io/richardcase/ubuntu-bionic-test:cloudimage_v0.0.1"
	kernelImage = "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11"
	fqdnFormat  = "%s.fruitcase"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	rlog.Configure(&rlog.Config{
		Verbosity: 10,
		Format:    "text",
		Output:    "stderr",
	})
	logger := rlog.GetLogger(ctx)
	logger.Infof("reignite dev-helper, using containerd socket: %s", socketPath)

	keyData, err := ioutil.ReadFile(sshKeyPath)
	if err != nil {
		panic(err)
	}

	logger.Infof("Running createvm plan to create %d microvms", numNodes)
	for i := 0; i < numNodes; i++ {
		runCreateVMPlanTest(ctx, i, string(keyData), logger)
	}

	logger.Info("finished creating microvms")

	cancel()
}

func runCreateVMPlanTest(ctx context.Context, nodeNum int, sshKey string, logger *logrus.Entry) {
	macMeta := "AA:FF:00:00:00:01"
	hostname := fmt.Sprintf("mvm%d", nodeNum)

	cfg := &config.Config{
		ConfigFilePath: "",
		Logging: rlog.Config{
			Verbosity: 9,
			Format:    "text",
		},
		GRPCAPIEndpoint:      "",
		HTTPAPIEndpoint:      "",
		CtrSnapshotterKernel: defaults.ContainerdKernelSnapshotter,
		CtrSnapshotterVolume: defaults.ContainerdVolumeSnapshotter,
		CtrSocketPath:        socketPath,
		CtrNamespace:         defaults.ContainerdNamespace,
		FirecrackerBin:       "/home/richard/bin/firecracker",
		FirecrackerDetatch:   false,
		FirecrackerUseAPI:    true,
		StateRootDir:         defaults.StateRootDir,
		ParentIface:          "enp1s0",
		DisableReconcile:     false,
		DisableAPI:           false,
		ResyncPeriod:         0,
	}

	vmid, _ := models.NewVMID(hostname, namespace)

	userdata, err := getUserMetadata(hostname, sshKey)
	if err != nil {
		panic(err)
	}
	hostmetadata, err := getHostMetadata(vmid, hostname)
	if err != nil {
		panic(err)
	}

	spec := &models.MicroVM{
		ID:      *vmid,
		Version: 0,
		Spec: models.MicroVMSpec{
			VCPU:       2,
			MemoryInMb: 2048,
			Kernel: models.Kernel{
				Image:            models.ContainerImage(kernelImage),
				CmdLine:          "console=ttyS0 reboot=k panic=1 pci=off i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd ds=nocloud-net;s=http://169.254.169.254/latest/",
				Filename:         "vmlinux",
				AddNetworkConfig: true,
			},
			Initrd: &models.Initrd{
				Image:    models.ContainerImage(kernelImage),
				Filename: "initrd-generic",
			},
			Volumes: models.Volumes{
				{
					ID:         "root",
					IsRoot:     true,
					IsReadOnly: false,
					MountPoint: "/",
					Source: models.VolumeSource{
						Container: &models.ContainerVolumeSource{
							Image: models.ContainerImage(rootfsImage),
						},
					},
				},
			},
			NetworkInterfaces: []models.NetworkInterface{
				{
					GuestDeviceName:       "eth0",
					GuestMAC:              macMeta,
					Type:                  models.IfaceTypeTap,
					AllowMetadataRequests: true,
					Address:               "169.254.0.1/16",
				},
				{
					GuestDeviceName:       "eth1",
					Type:                  models.IfaceTypeMacvtap,
					AllowMetadataRequests: false,
				},
			},
			Metadata: map[string]string{
				"meta-data": hostmetadata,
				"user-data": userdata,
			},
		},
	}

	ports, err := inject.InitializePorts(cfg)
	if err != nil {
		panic(err)
	}
	execCtx := portsctx.WithPorts(ctx, ports)

	input := &plans.CreatePlanInput{
		StateDirectory: cfg.StateRootDir,
		VM:             spec,
	}

	start := time.Now()

	plan := plans.MicroVMCreatePlan(input)

	actuator := planner.NewActuator()
	if _, err := actuator.Execute(execCtx, plan, "1234567890"); err != nil {
		panic(err)
	}

	if err := ports.Provider.Start(ctx, spec.ID.String()); err != nil {
		panic(err)
	}

	elapsed := time.Since(start)
	log.Printf("create & start microvm took %s", elapsed)
}

func getUserMetadata(hostname, sshkey string) (string, error) {
	runCommands := []string{
		"dhclient -r",
		"dhclient",
	}

	userData := &cloudinit.UserData{
		Users: &[]cloudinit.User{
			{
				Name:              "root",
				SSHAuthorizedKeys: &[]string{sshkey},
			},
		},
		HostName:      ptr.String(hostname),
		Fqdn:          ptr.String(fmt.Sprintf(fqdnFormat, hostname)),
		DisableRoot:   ptr.Bool(false),
		PackageUpdate: ptr.Bool(false),
		FinalMessage:  ptr.String("The reignited booted system is good to go after $UPTIME seconds"),
		// WriteFiles:      nil,
		RunCommands: &runCommands,
	}

	md, err := yaml.Marshal(userData)
	if err != nil {
		return "", fmt.Errorf("marshalling cloud-init userdata: %w", err)
	}

	userDataStr := fmt.Sprintf("#cloud-config\n%s", string(md))

	return base64.StdEncoding.EncodeToString([]byte(userDataStr)), nil
}

func getHostMetadata(vmid *models.VMID, hostname string) (string, error) {
	meta := &cloudinit.Metadata{
		InstanceID:    vmid.String(),
		LocalHostname: hostname,
		Platform:      "liquid_metal",
	}

	md, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshalling cloud-init metadata: %w", err)
	}

	return base64.StdEncoding.EncodeToString(md), nil
}
