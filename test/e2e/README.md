## E2E tests

The end to end tests are written in Go.
They are fairly simple, and currently cover a simple CRUD happy path.
We aim to test as much complexity as possible in lighter weight unit and
integration tests.

There are several ways to run the end to end tests.

### Requirements

The tests start the `containerd` found on the `PATH` and need it to be
containerd v2 or later. The test setup checks the version and fails if an older
containerd is found.

The private registry test also needs these to be on the `PATH`:
- [`zot`][zot]: this is the registry which the test starts. The `minimal` build
  is enough.
- [`skopeo`][skopeo]: this is used to copy the images to the registry.

### Tests

- `TestE2E`: covers the CRUD happy path using images from a public registry.
- `TestE2EPrivateRegistry`: creates a MicroVM using images from a private
  registry. The test starts a registry which requires authentication on
  `127.0.0.1:5050`, copies the kernel and root volume images to it, and writes
  the `hosts.toml` with the credentials to the directory which flintlockd was
  started with (`--containerd-hosts-dir`). The test also reads the log of the
  registry, to check that flintlockd pulled the manifests and the blobs of the
  images from it.
- `TestE2EPrivateRegistryNoCredentials`: uses the same registry and images, but
  does not write the `hosts.toml`. The MicroVM must stay `PENDING` while the
  retry count goes up, and firecracker must not be started. The test reads the
  log of the registry, to check that it refused the requests of flintlockd with
  a `401`.

Each of the tests does its own setup and teardown, so that the private registry
tests start with a containerd which does not have any of the images.

### In your local environment

```
make test-e2e
```

This will run the tests directly on your host with minimal fuss.
You must ensure that you have installed all the dependencies per the
[Quick-Start guide][quick-start].

### In a local docker container

```
make test-e2e-docker
```

This will run the tests in a Docker container running on your host machine.
Note that due to the nature of flintlock, the container will be run with
high privileges and will share some devices and process memory with the host.

### In an Equinix device

```bash
export METAL_AUTH_TOKEN=<your token>
export EQUINIX_ORG_ID=<your org id>
make test-e2e-metal
```

This will use the tool at `./test/tools/run.py` to create a new project and device
with the credentials provided above, and then run the tests within that device.

This exact command will run tests against main of the upstream branch, and only with
minimal configuration. Read the tool [usage docs](/test/tools/README.md) for information
on how to configure and use the tool in your development.

### In GitHub Actions on hosted runners

The `hosted e2e` workflow runs the e2e suite directly on `ubuntu-latest` GitHub
hosted runners. It is available via `workflow_dispatch`.

The workflow prepares the runner by installing the host packages required by the
test harness, installing pinned releases of containerd, zot and Firecracker, and
checking that `/dev/kvm` exists. The versions of these can be changed with the
workflow inputs. The
tests are run with `sudo` because they create loop devices, devicemapper
thinpools and a `fl-e2e-br0` bridge for the microVM TAP interfaces, write
containerd configuration under `/etc`, and manage runtime state under `/run`
and `/var/lib`.

### Configuration

There are a couple of custom test flags which you can set to alter the behaviour
of the tests.

At the time of writing these are:
- `skip.setup.thinpool`: skips the setup of devicemapper thinpool.
- `thinpool`: set the name of a custom thinpool to create and/or use instead.
- `skip.delete`: skip the Delete step of the tests and leave the mVMs around for debugging.
  This will also leave containerd and flintlockd running. All cleanup will be manual.
- `skip.teardown`: skip stopping containerd and flintlockd processes.
  Like `skip.delete`, this also leaves the `fl-e2e-br0` bridge behind.
- `level.containerd`: set the containerd log level.
- `level.flintlockd`: set the flintlockd log level.

You can pass in these flags to the test like so:

```bash
./test/e2e/test.sh -level.flintlockd=9
```

All the flags can be found at [`params.go`](/test/e2e/utils/params.go).

The `skip.delete` and `skip.teardown` flags leave the environment of a test
running, so the tests which come after it are skipped. Use `-run` to choose the
test to debug:

```bash
./test/e2e/test.sh -run '^TestE2EPrivateRegistry$' -skip.delete
```

### Following the progress

The output of the tests also has the logs of containerd, flintlockd and the
registry in it. The tests mark their own lines so that they can be found:

- `TEST STEP:` is the start of a step of a test.
- `TEST INFO:` is a detail of the step, such as the image which is copied or
  the state of the MicroVM when it changes.

To only see the progress of the tests:

```bash
./test/e2e/test.sh 2>&1 | grep -E 'TEST (STEP|INFO):|^(=== RUN|--- |ok|FAIL)'
```

[zot]: https://zotregistry.dev
[skopeo]: https://github.com/containers/skopeo
