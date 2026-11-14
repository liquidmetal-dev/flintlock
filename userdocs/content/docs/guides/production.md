---
weight: 4
title: Production
---

The [getting started tutorial](/docs/getting-started/) runs `flintlockd` with
`--insecure` and a loop-device thinpool. That's fine for trying flintlock out,
but a production host needs TLS, authentication and real storage. This guide
covers the settings to change.

## Provision the host

`flintlock-provision` sets up a host in one step. It installs the apt
packages, Firecracker, Cloud Hypervisor, containerd and `flintlockd`, creates
the devmapper thinpool, writes the `flintlockd` config file and starts the
systemd services. Download it for your architecture from the
[latest release][releases], then run:

```bash
sudo flintlock-provision all \
  --disk /dev/sdb \
  --grpc-address 192.168.1.66 \
  --parent-iface eth0 \
  --flintlock-config-file ./flintlockd-overrides.yaml
```

| Flag | Description |
| ---- | ----------- |
| `--disk` | A blank, unpartitioned disk for the direct-lvm thinpool. Don't use `--dev` in production: it creates loop-device thinpools instead. |
| `--grpc-address`, `--grpc-port` | The address and port for the gRPC API. The defaults are the host's local IPv4 address and `9090`. |
| `--parent-iface` or `--bridge` | The host interface (or Linux bridge) that microVM network interfaces attach to. |
| `--flintlock-config-file` | A YAML file of `flintlockd` settings that are merged into the generated config. Use it for the TLS and auth settings below. |
| `--insecure` | Starts `flintlockd` without TLS or basic auth. Don't use it in production. |

Component versions can be pinned with the `FLINTLOCK`, `CONTAINERD`,
`FIRECRACKER` and `CLOUD_HYPERVISOR` environment variables. Run
`flintlock-provision all --help` for the other flags.

## The config file

`flintlockd` reads `/etc/opt/flintlockd/config.yaml`, which is the file
`flintlock-provision` writes. Each key is the name of a `flintlockd run` flag
without the leading `--`, for example:

```yaml
grpc-endpoint: 192.168.1.66:9090
parent-iface: eth0
containerd-socket: /run/containerd/containerd.sock
tls-cert: /etc/opt/flintlockd/certs/server.pem
tls-key: /etc/opt/flintlockd/certs/server-key.pem
basic-auth-token: <token>
```

A flag on the command line overrides the same key in the file. Restart the
service after you change the file:

```bash
sudo systemctl restart flintlockd
```

## Enable TLS

TLS is on unless you pass `--insecure`. `flintlockd` won't start without a
certificate and key:

```yaml
tls-cert: /etc/opt/flintlockd/certs/server.pem
tls-key: /etc/opt/flintlockd/certs/server-key.pem
```

The server only accepts TLS 1.3. The certificate must be valid for the
address that clients use to connect, so include that IP address or host name
in its subject alternative names.

### Require client certificates

To make every client present a certificate signed by your CA (mutual TLS),
add:

```yaml
tls-client-validate: true
tls-client-ca: /etc/opt/flintlockd/certs/ca.pem
```

For a test environment, [`hack/scripts/gen_local_certs.sh`][gen-certs]
creates a CA, a server certificate and a client certificate. Use certificates
from your own PKI in production.

## Enable basic auth

Set a shared token:

```yaml
basic-auth-token: <token>
```

Clients must then send the base64-encoded token in the `authorization`
metadata of every request:

```text
authorization: basic <base64 of token>
```

The token is sent in every request, so only use basic auth together with TLS.

## Lock down optional APIs

These are off by default. Only turn them on when you need them.

| Flag | What it exposes |
| ---- | --------------- |
| `--enable-exec-api` | Runs commands inside any microVM that was created with `allow_guest_agent`. Anyone who can call the API can run any command in those microVMs, as any guest user. |
| `--enable-ssh-proxy-api` | Tunnels SSH connections to a guest's `sshd` through the guest agent. `sshd` still handles authentication. |
| `--enable-http` | Serves the HTTP gateway on `--http-endpoint` (default `localhost:8090`). The gateway connects to the gRPC endpoint without TLS, so it doesn't work when TLS is enabled. |
| `--debug-endpoint` | Serves Go `pprof` data. If you turn it on, bind it to `localhost`. |

## Run as a systemd service

`flintlock-provision` installs [`flintlockd.service`][service], which restarts
`flintlockd` if it exits. Firecracker and Cloud Hypervisor processes are
detached from `flintlockd` by default (`--firecracker-detach` and
`--cloudhypervisor-detach`), and the unit uses `KillMode=process`, so running
microVMs keep going while `flintlockd` restarts or upgrades.

Check the service and its logs with:

```bash
sudo systemctl status flintlockd
sudo journalctl -u flintlockd -f
```

## Pull from private registries

`flintlockd` reads registry credentials and mirrors from containerd's
`certs.d` layout, one `hosts.toml` per registry, in
`/etc/containerd/certs.d`. Use `--containerd-hosts-dir` to point at another
directory. See [MicroVM images](/docs/guides/images/) for more about images.

## Monitor the host

Run the [metrics exporter](/docs/guides/metrics/) so Prometheus can scrape
microVM metrics from the host.

[releases]: https://github.com/liquidmetal-dev/flintlock/releases/latest
[gen-certs]: https://github.com/liquidmetal-dev/flintlock/blob/main/hack/scripts/gen_local_certs.sh
[service]: https://github.com/liquidmetal-dev/flintlock/blob/main/flintlockd.service
