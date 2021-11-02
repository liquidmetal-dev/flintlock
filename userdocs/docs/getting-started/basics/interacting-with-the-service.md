---
sidebar_position: 5
---

# Interacting with the service

We recommend using one of the following tools to send requests to the Flintlock server.
There are both GUI and a CLI option.

## grpc-client-cli

Install the [grpcurl][grpcurl].

Use the [`./hack/scripts/send.sh`][payload-example-send] script.

### Example

To created a MicroVM:

```
./hack/scripts/send.sh \
  --method CreateMicroVM
```

In the terminal where you started the Flintlock server, you should see in the logs that the MircoVM
has started.

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
example][payload-example-create].  Click the green `>` in the centre of the
screen. The response should come immediately.

In the terminal where you started the Flintlock server, you should see in the
logs that the MircoVM has started.

To delete the MircoVM, select the `DeleteMicroVM` endpoint in the left-hand
menu.  Replace the sample request JSON in the left editor panel with [this
example][payload-example-delete].  Take care to ensure the values match those
of the MicroVM you created earlier.  Click the green `>` in the centre of the
screen. The response should come immediately.

**Note: there are example payloads for other endpoints, but not all are
implemented at present.**

[grpcurl]: https://github.com/fullstorydev/grpcurl
[bloomrpc]: https://github.com/uw-labs/bloomrpc
[payload-example-send]: https://github.com/weaveworks/flintlock/blob/main/hack/scripts/send.sh
[payload-example-create]: https://github.com/weaveworks/flintlock/blob/main/hack/scripts/payload/CreateMicroVM.json
[payload-example-delete]: https://github.com/weaveworks/flintlock/blob/main/hack/scripts/payload/DeleteMicroVM.json
