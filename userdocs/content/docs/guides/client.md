---
weight: 5
title: Go client
---

The gRPC API is published as two Go modules:

- `github.com/liquidmetal-dev/flintlock/api` holds the generated gRPC clients
  and message types.
- `github.com/liquidmetal-dev/flintlock/client` holds helpers for clients,
  such as cloud-init metadata keys and types.

Neither module has tagged releases, so `@latest` resolves to the newest
commit on `main`:

```bash
go get github.com/liquidmetal-dev/flintlock/api@latest
go get github.com/liquidmetal-dev/flintlock/client@latest
```

If the build fails with an `ambiguous import` error for a
`google.golang.org/genproto/googleapis/...` package, upgrade the old
`genproto` module:

```bash
go get google.golang.org/genproto@latest
```

## Create and get a microVM

This example connects to a host that has TLS and basic auth turned on, as
described in the [production guide](/docs/guides/production/). It creates a
microVM from OCI images and then reads back its state.

```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit"
)

func main() {
	caPEM, err := os.ReadFile("ca.pem")
	if err != nil {
		log.Fatal(err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	creds := credentials.NewTLS(&tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	})

	conn, err := grpc.NewClient("192.168.1.66:9090", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := mvmv1.NewMicroVMClient(conn)

	token := base64.StdEncoding.EncodeToString([]byte(os.Getenv("FLINTLOCK_TOKEN")))
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "basic "+token)

	userData := "#cloud-config\nhostname: my-microvm\n"

	resp, err := client.CreateMicroVM(ctx, &mvmv1.CreateMicroVMRequest{
		Microvm: &types.MicroVMSpec{
			Id:         "my-microvm",
			Namespace:  "default",
			Vcpu:       2,
			MemoryInMb: 2048,
			Kernel: &types.Kernel{
				Image:            "ghcr.io/liquidmetal-dev/firecracker-kernel:6.1",
				Filename:         new("vmlinux"),
				AddNetworkConfig: true,
			},
			RootVolume: &types.Volume{
				Id: "root",
				Source: &types.VolumeSource{
					ContainerSource: new("ghcr.io/liquidmetal-dev/ubuntu:24.04"),
				},
			},
			Metadata: map[string]string{
				cloudinit.UserdataKey: base64.StdEncoding.EncodeToString([]byte(userData)),
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	uid := resp.GetMicrovm().GetSpec().GetUid()
	fmt.Println("created", uid)

	got, err := client.GetMicroVM(ctx, &mvmv1.GetMicroVMRequest{Uid: uid})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("state:", got.GetMicrovm().GetStatus().GetState())
}
```

Some points about the example:

- `grpc.NewClient` doesn't connect straight away. The first RPC opens the
  connection.
- The basic auth token goes base64-encoded in the `authorization` metadata,
  after `basic `. Leave it out if the server has no `--basic-auth-token`.
- For a server started with `--insecure`, use
  `grpc.WithTransportCredentials(insecure.NewCredentials())` from
  `google.golang.org/grpc/credentials/insecure` instead of TLS.
- For mutual TLS, add your client certificate to `Certificates` in the
  `tls.Config`.
- Metadata values must be base64-encoded. The `cloudinit` package has the key
  names (`UserdataKey`, `InstanceDataKey`, `VendorDataKey` and
  `NetworkConfigDataKey`). The `cloudinit/userdata` and `cloudinit/instance`
  packages have types you can marshal to YAML instead of writing it by hand.
- `CreateMicroVM` returns once flintlock accepts the spec, and the microVM
  starts in the background. Poll `GetMicroVM` until the state changes from
  `PENDING` to `CREATED` (or `FAILED`).

## Other services

The `api` module has clients for the other services too:

| Package | Client | Needs |
| ------- | ------ | ----- |
| `api/services/microvm/v1alpha1` | `NewMicroVMClient` | |
| `api/services/microvmexec/v1alpha1` | `NewMicroVMExecClient` | `--enable-exec-api` on the server |
| `api/services/microvmsshproxy/v1alpha1` | `NewMicroVMSSHProxyClient` | `--enable-ssh-proxy-api` on the server |

Exec and SSH also need a microVM created with `AllowGuestAgent: true` that
runs the [guest agent](https://github.com/liquidmetal-dev/guest-agent). See
the [gRPC reference](/docs/grpc/) for every message and field.

## Connecting through a proxy

To reach a flintlock host through an HTTP `CONNECT` proxy, add the
`WithProxy` dial option from `github.com/liquidmetal-dev/flintlock/client/grpc`:

```go
proxyURL, _ := url.Parse("http://proxy.example.com:3128")

conn, err := grpc.NewClient("192.168.1.66:9090",
	grpc.WithTransportCredentials(creds),
	flgrpc.WithProxy(proxyURL),
)
```
