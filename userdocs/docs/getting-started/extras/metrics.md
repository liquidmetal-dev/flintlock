# Start metrics exporter

Flintlock has a metrics exporter called `flintlock-metrics`. It listens on an
HTTP port and serves Prometheus compatible output.

```
sudo ./bin/flintlock-metrics serve \
  --containerd-socket=/run/containerd-dev/containerd.sock \
  --http-endpoint=0.0.0.0:8000
```

Available endpoints:

* `/machine/uid/{uid}`: Metrics for a specific MicroVM.
* `/machine/{namespace}/{name}`: Metrics for all MicroVMs with given name and namespace.
* `/machine/{namespace}`: Metrics for all MicroVMs under a specific Namespace.
* `/machine`: Metrics for all MicroVMs from all Namespaces.

For testing/development, there is a minimal docker compose setup under `hack/scripts/monitoring/metrics`.
