---
sidebar_position: 4
---

# Set up and start flintlock

```bash
go mod download
make build

NET_DEVICE=$(ip route show | awk '/default/ {print $5}')

sudo ./bin/flintlockd run \
  --containerd-socket=/run/containerd-dev/containerd.sock \
  --parent-iface="${NET_DEVICE}"
```

If you're running `flintlockd` from within a Vagrant VM and wish to call the gRPC API from your host machine then you need to run `flintlockd` with the `--grpc-endpoint=0.0.0.0:9090` flag, otherwise the connection will be rejected.

You should see it start successfully with similar output:
```
INFO[0000] flintlockd, version=undefined, built_on=undefined, commit=undefined
INFO[0000] flintlockd grpc api server starting
INFO[0000] starting microvm controller
INFO[0000] starting microvm controller with 1 workers    controller=microvm
INFO[0000] resyncing microvm specs                       controller=microvm
INFO[0000] Resyncing specs                               action=resync controller=microvm namespace=ns
INFO[0000] starting event listener                       controller=microvm
INFO[0000] Starting workersnum_workers1                  controller=microvm
```

