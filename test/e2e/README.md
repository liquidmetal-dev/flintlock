## E2E tests

The end to end tests are written in Go.
They are fairly simple, and currently cover a simple CRUD happy path.
We aim to test as much complexity as possible in lighter weight unit and
integration tests.

There are several ways to run the end to end tests.

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
minimal configuration. Read the tool [usage docs](test/tools/README.md) for information
on how to configure and use the tool in your development.

### Configuration

There are a couple of custom test flags which you can set to alter the behaviour
of the tests.

At the time of writing these are:
- `skip.setup.thinpool`: skips the setup of devicemapper thinpools.
- `skip.delete`: skip the Delete step of the tests and leave the mVMs around for debugging.
  This will also leave containerd and flintlockd running. All cleanup will be manual.
- `skip.teardown`: skip stopping containerd and flintlockd processes.
- `level.containerd`: set the containerd log level.
- `level.flintlockd`: set the flintlockd log level.

You can pass in these flags to the test like so:

```bash
./test/e2e/test.sh -level.flintlockd=9
```

All the flags can be found at [`params.go`](test/e2e/utils/params.go).
