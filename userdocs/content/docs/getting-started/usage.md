---
weight: 6
title: Using the service
---

We recommend using one of the following tools to send requests to the Flintlock server.
There are GUI and CLI options.

## hammertime

[hammertime][ht] is a simple tool which gives a nice wrapper around what is essentially
a `curl`. Use this if you need no bells and whistles and would rather manipulate your
own `json`.

### Example

```bash
hammertime create -a 192.168.1.66:9090 -n my-microvm -ns my-namespace

hammertime get -a 192.168.1.66:9090 -i <UUID>

hammertime create -a 192.168.1.66:9090 -f spec.json

hammertime delete -a 192.168.1.66:9090 --all
```

## fl

[fl][fl] aims to provide a fuller user experience, with formatted output and some
extra features for handling your MicroVMs.

`fl` connects without TLS and doesn't send an auth token, so use it with a
`flintlockd` that was started with `--insecure`, as in this tutorial.

### Example

Create a microVM. The kernel and root volume are OCI images, and `fl` uses
these images by default if you leave the flags out:

```bash
fl microvm create --host 192.168.1.66:9090 --name my-microvm \
  --kernel-image ghcr.io/liquidmetal-dev/firecracker-kernel:6.1 \
  --root-image ghcr.io/liquidmetal-dev/ubuntu:24.04
```

The output includes the `uid` of the new microVM. List the microVMs on the
host, or get one by its `uid`:

```bash
fl microvm get --host 192.168.1.66:9090

fl microvm get --host 192.168.1.66:9090 <uid>
```

Delete the microVM:

```bash
fl microvm delete --host 192.168.1.66:9090 <uid>
```

Run `fl microvm create --help` for the other options, including network
interfaces, cloud-init metadata and extra volumes.

### Running commands and SSH

`fl` can also run a command in a microVM, or open an SSH session to it, using
the guest agent. To do this:

1. Start `flintlockd` with `--enable-exec-api` for exec, `--enable-ssh-proxy-api`
   for SSH, or both.
1. Create the microVM with `--allow-guest-agent`.
1. Use a root image that runs the [guest agent][guest-agent]. For SSH, the
   image must also run `sshd`.

```bash
fl microvm exec --host 192.168.1.66:9090 <uid> -- uname -a

fl microvm ssh --host 192.168.1.66:9090 <uid> -- -l ubuntu
```

Before you run a command, wait until the status of the microVM has the
`GuestAgentReady` condition with the status `TRUE`. flintlock pings the guest
agent after it starts the VMM, until the agent answers or
`--guest-agent-ready-timeout` (default `60s`) expires. When the agent does not
answer in time, the condition is `FALSE` with the reason `PingTimeout`, and it
stays `FALSE` until the VMM starts again.

The `VMMRunning` condition shows that the VMM process runs. flintlockd waits up
to `--vmm-start-timeout` (default `30s`) for it.

Arguments after `--` in `fl microvm ssh` are passed to `ssh`.

## grpc-client-cli

For the most bare-bones experience, use [grpcurl][grpcurl] and the [`./hack/scripts/send.sh`][payload-example-send] script.

### Example

To create a MicroVM:

```bash
./hack/scripts/send.sh \
  --method CreateMicroVM
```

## BloomRPC

[BloomRPC][bloomrpc] is a GUI tool to test gRPC endpoints.

### Import

To import Flintlock protos into the Bloom GUI:

1. Click `Import Paths` on the left-hand menu bar and add `<absolute-repo-path>/api` to the list
1. Click the import `+` button and select `flintlock/api/services/microvm/v1alpha1/microvms.proto`

All available endpoints will be visible in a nice tree view.

### Example

To create a MircoVM, select the `CreateMicroVM` endpoint in the left-hand menu.
Replace the sample request JSON in the left editor panel with [this
example][payload-example-create]. Click the green `>` in the centre of the
screen. The response should come immediately.

In the terminal where you started the Flintlock server, you should see in the
logs that the MircoVM has started.

To delete the MircoVM, select the `DeleteMicroVM` endpoint in the left-hand
menu. Replace the sample request JSON in the left editor panel with [this
example][payload-example-delete]. Take care to ensure the values match those
of the MicroVM you created earlier. Click the green `>` in the centre of the
screen. The response should come immediately.

**Note: there are example payloads for other endpoints, but not all are
implemented at present.**

[grpcurl]: https://github.com/fullstorydev/grpcurl
[bloomrpc]: https://github.com/uw-labs/bloomrpc
[payload-example-send]: https://github.com/liquidmetal-dev/flintlock/blob/main/hack/scripts/send.sh
[payload-example-create]: https://github.com/liquidmetal-dev/flintlock/blob/main/hack/scripts/payload/CreateMicroVM.json
[payload-example-delete]: https://github.com/liquidmetal-dev/flintlock/blob/main/hack/scripts/payload/DeleteMicroVM.json
[ht]: https://github.com/warehouse-13/hammertime
[fl]: https://github.com/liquidmetal-dev/fl
[guest-agent]: https://github.com/liquidmetal-dev/guest-agent
