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

The tests run with each of the microvm providers, and need the VMM of each of
them on the `PATH`:

| Provider | VMM binary | Installed by |
|---|---|---|
| `firecracker` | `firecracker` | `hack/scripts/provision.sh firecracker` |
| `cloudhypervisor` | `cloud-hypervisor-static` | `hack/scripts/provision.sh cloudhypervisor` |

The test setup fails if one of them is not found. Use the `providers` flag to
run the tests on a host which does not have both, see
[Configuration](#configuration).

The private registry test also needs these to be on the `PATH`:
- [`zot`][zot]: this is the registry which the test starts. The `minimal` build
  is enough.
- [`skopeo`][skopeo]: this is used to copy the images to the registry.

The overlay boot test needs what flintlockd needs to boot a MicroVM from an
overlay root, see the [volumes guide][volumes-guide]:

| What | Why | How to get it |
|---|---|---|
| The state directory `/var/lib/flintlock` on a filesystem with reflinks | flintlockd clones the writable disk of each MicroVM from a template with `FICLONE` | `flintlock-provision state-dir --dir /var/lib/flintlock --loop-file <file> --size 20`, which formats a sparse file as XFS and mounts it there. A directory which is on XFS with `reflink=1` or on Btrfs needs nothing |
| The pinned `mkfs.erofs` at `/opt/flintlock/bin/mkfs.erofs` | flintlockd builds the base of an image with it | `flintlock-provision erofs-utils`, or `hack/scripts/build_erofs_utils.sh` and `install` the binary there |
| `mkfs.ext4` on the `PATH` | flintlockd formats the template of the writable disks with it | The `e2fsprogs` package |
| `e2fsck` and `debugfs` on the `PATH` | The test reads the upper directory of a writable disk with them | The `e2fsprogs` package |
| The image of the overlay initrd | The guest boots from it. flintlockd pulls it when the MicroVM is created | The `overlay.initrd.image` flag, see [Configuration](#configuration) |

The test is skipped when the `overlay.initrd.image` flag is not set, so the
other tests run on a host which has none of these. flintlockd starts without
them as well, and refuses a MicroVM with an overlay root with an error which
names what is missing.

The image of the initrd is published with each release of flintlock, as
`ghcr.io/liquidmetal-dev/flintlock-overlay-init:<version>`. To test the initrd
of a checkout, build it and push it to a registry which flintlockd can pull
from:

```bash
make build-overlay-initrd VERSION=dev
docker build -f hack/packaging/flintlock-overlay-init.Dockerfile -t 127.0.0.1:5000/flintlock-overlay-init:dev out
docker push 127.0.0.1:5000/flintlock-overlay-init:dev
```

containerd pulls from `127.0.0.1` with plain HTTP, so a registry on the host
needs no TLS. The [hosted workflow](#in-github-actions-on-hosted-runners)
does this with `zot` on port 5000.

### Tests

- `TestE2E`: covers the CRUD happy path using images from a public registry.
  It has a subtest for each of the providers, such as `TestE2E/cloudhypervisor`.
  The subtests share one flintlockd which has all of the providers enabled, and
  each MicroVM names its provider in its spec.
- `TestE2EPrivateRegistry`: creates a MicroVM using images from a private
  registry. The test starts a registry which requires authentication on
  `127.0.0.1:5050`, copies the kernel and root volume images to it, and writes
  the `hosts.toml` with the credentials to the directory which flintlockd was
  started with (`--containerd-hosts-dir`). The test also reads the log of the
  registry, to check that flintlockd pulled the manifests and the blobs of the
  images from it.
- `TestE2EPrivateRegistryNoCredentials`: uses the same registry and images, but
  does not write the `hosts.toml`. The MicroVM must stay `PENDING` while the
  retry count goes up, and no VMM must be started. The test reads the
  log of the registry, to check that it refused the requests of flintlockd with
  a `401`.
- `TestE2ECloudHypervisorKernelWithoutPVH`: creates a MicroVM with the
  cloudhypervisor provider and a kernel which Cloud Hypervisor cannot boot. The
  VMM must not be running, the console must not have the boot marker, and the
  stderr of the VMM must say that the kernel has no PVH entry point. It shows
  that the checks of the other tests do not pass when the guest does not boot.
  The test does not look at the state of the MicroVM, which is `CREATED`
  ([#1263][issue-1263]). It is skipped when `cloudhypervisor` is not one of the
  providers.
- `TestE2EGuestWithoutInit`: creates a MicroVM whose root volume has no init,
  with each of the providers. The kernel of the guest boots, mounts the root
  volume, panics and reboots, which the test reads from the console. The
  console must not have the boot marker. Firecracker exits when the guest
  reboots. Cloud Hypervisor keeps running and starts the guest again, so the
  MicroVM is `CREATED` and has a VMM which runs, and only the boot marker tells
  it from a MicroVM that runs.
- `TestE2EOverlayBoot`: boots the unmodified `ghcr.io/liquidmetal-dev/ubuntu:24.04`
  image from an overlay root. It has a subtest for each of the kernels of
  [Kernel images](#kernel-images): `firecracker-5.10`, `firecracker-6.1` and
  `cloudhypervisor-6.2`. Each subtest creates two MicroVMs from the image, in
  turn, and deletes both. The subtests share one flintlockd. The test is
  skipped without the `overlay.initrd.image` flag, and a subtest is skipped
  when its provider is not one of the providers. See
  [What shows that a root is an overlay](#what-shows-that-a-root-is-an-overlay).

Each of the tests does its own setup and teardown, so that the private registry
tests start with a containerd which does not have any of the images. The
private registry tests use the first of the providers.

### What shows that a MicroVM is running

That a MicroVM is `CREATED` and that its VMM is running does not show that its
guest has booted. A VMM can run for a while with a guest which has no kernel
that it can boot, or with a kernel which cannot mount its root volume. The
tests which create a MicroVM check all of these:

| Check | What it rules out |
|---|---|
| The state directory has the pid file of the provider, and the process is running | The VMM was not started |
| `/proc/<pid>/exe` is the VMM binary of the provider | flintlockd started the VMM of another provider |
| The state of the MicroVM is `CREATED` | flintlockd has not finished, or has failed |
| The console of the guest has the boot marker of the MicroVM | The guest did not boot, did not mount its root volume, or did not get the metadata of the MicroVM |
| The pid of the VMM is the same before and after the marker was found | The marker is from a VMM which has stopped since |

The boot marker is the `final_message` of cloud-init in the user-data of the
test MicroVMs:

```text
flintlock-e2e boot ok <namespace>/<name> uptime=<seconds>
```

cloud-init writes it to the console when it has run all of its stages, and
fills in the uptime. The console of the guest is the `<provider>.stdout` file in
the state directory of the MicroVM.

When a test fails, it logs the end of the console, the stderr and the log of
the VMM, and deletes its MicroVMs. The whole files are saved if the tests are
run with the `artefacts.dir` flag, see
[Saving the files of the MicroVMs](#saving-the-files-of-the-microvms).

### What shows that a root is an overlay

A MicroVM with an overlay root has two drives in place of its root volume: a
read-only base, which every MicroVM of the image shares, and a writable disk of
its own. The init of the overlay initrd mounts the base as EROFS, the writable
disk as ext4, joins them with overlayfs and hands over to the init of the
image. On the host, the state directory of the MicroVM has
`volumes/root.base`, a link to `/var/lib/flintlock/bases/<sha256>.img`, and
`volumes/root.rw`, a clone of `/var/lib/flintlock/templates/rw-<size>.img`.

`TestE2EOverlayBoot` takes its evidence from the host and from the console of
the guest. The state which flintlockd reports is logged, it is not evidence
([#1263][issue-1263]). The checks are in addition to the ones of
[What shows that a MicroVM is running](#what-shows-that-a-microvm-is-running):

| Check | What it shows |
|---|---|
| The console has the line of the overlay init, with `base_fs=erofs` and `rw_fs=ext4` on two devices | The root of the guest is the overlay |
| `root.rw` has more allocated blocks than a fresh clone of the template, which the test makes | The guest writes to its writable disk, and not to the base |
| `debugfs -R "ls -l /upper"` on a snapshot of `root.rw` with the journal replayed lists `etc`, `var` or `tmp` | The guest writes the files of the overlay to the upper directory of its writable disk |
| The console has the boot marker of cloud-init | cloud-init runs all of its stages on an overlay root. Its `resizefs` module finds an overlay and is expected to skip it, what it writes about it is logged |
| `root.base` is a symlink to a file in `bases/` with mode `0400`, whose sha256 is its name | The base is the one flintlockd built, and nothing wrote to it |
| The sha256 and the modification time of the base are the same after the MicroVMs ran and after they were deleted | The guests did not change the base |
| `root.base` of the second MicroVM resolves to the same inode as the one of the first | The second MicroVM reuses the base |
| `root.rw` of the second MicroVM is another inode than the one of the first | Each MicroVM has a writable disk of its own |
| After the delete, `root.rw` and `root.base` of both MicroVMs are gone, and the base is still there | The delete removes the writable disk and the link, and keeps the base for the next MicroVM |

The line of the overlay init on the console has the contract version, both
devices and their filesystems:

```text
flintlock-overlay-init: contract=1 base=/dev/vda base_fs=erofs rw=/dev/vdb rw_fs=ext4 init=/sbin/init
```

The writable disks of the test are 2048 MB, so the template is
`templates/rw-2048.img`. The base and the template stay in the state
directory when the test is over, flintlockd removes a base which no MicroVM
uses after its retention period.

The blocks of `root.rw` are not compared with the ones of the template.
`mkfs.ext4` allocates the inode tables with `fallocate`, which XFS keeps as
unwritten extents, and XFS does not map unwritten extents into a reflink. A
fresh clone has far fewer blocks than the template (8 MB against 68 MB for
2048 MB), so the test clones the template itself, on the same filesystem, and
compares with that.

The listing of the upper directory is not read from `root.rw` itself.
`debugfs` reads an image as it is and does not replay the ext4 journal, so an
entry which the guest has only committed to the journal is not in the
listing. The test clones `root.rw` to a snapshot on the same filesystem, runs
`e2fsck -fy` on the snapshot, which replays the journal and repairs what a
snapshot of a disk in use has, and lists the snapshot with `debugfs`. The
listing has what the guest had committed when the snapshot was made. The
guest commits its journal every few seconds, so the test tries again until
the listing has one of the directories. The snapshot is removed.

### Kernel images

| MicroVMs | Kernel image | Why |
|---|---|---|
| `firecracker` | `ghcr.io/liquidmetal-dev/firecracker-kernel:6.1` | Firecracker describes the devices of the guest with ACPI. A kernel needs `CONFIG_PCI` to use the ACPI tables, which this one has |
| `cloudhypervisor` | `ghcr.io/liquidmetal-dev/cloudhypervisor-kernel-bin:5.12` | Cloud Hypervisor boots the kernel from its PVH entry point, and attaches the devices with virtio over PCI |
| `TestE2ECloudHypervisorKernelWithoutPVH` | `ghcr.io/liquidmetal-dev/flintlock-kernel:5.10.77` | The kernel has no PVH entry point, so Cloud Hypervisor refuses it |
| `TestE2EOverlayBoot/firecracker-5.10` | `ghcr.io/liquidmetal-dev/firecracker-kernel:5.10-20260929-1349` | The first build of the kernel with EROFS. The tag names the build and is never moved |
| `TestE2EOverlayBoot/firecracker-6.1` | `ghcr.io/liquidmetal-dev/firecracker-kernel:6.1-20260929-1349` | As above |
| `TestE2EOverlayBoot/cloudhypervisor-6.2` | `ghcr.io/liquidmetal-dev/cloudhypervisor-kernel:6.2-20260929-1411` | The first build of the kernel with EROFS, virtio-blk, ext4, overlayfs and initrd support. The kernel is `vmlinux.bin` at the root of the image |

`TestE2EGuestWithoutInit` uses the kernel image of the provider as the image of
the root volume as well. It has the kernel and nothing else, so it is a root
volume without an init.

The guests boot with the kernel command line which the provider of flintlockd
sets, the tests do not add to it.

A kernel with `CONFIG_ACPI` and without `CONFIG_PCI` does not boot with
Firecracker `v1.16`, see [#1262][issue-1262]. `flintlock-kernel:5.10.77` is
one of them, it was the kernel of the firecracker MicroVMs of the tests.

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

The image of the container cannot be built at the moment:
`test/docker/Dockerfile.e2e` needs `hack/scripts/bootstrap.sh`, which is not in
the repo. An image for the tests has to have what is listed in
[Requirements](#requirements).

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

The device does not have what the tests need at the moment, see
[Requirements](#requirements): it gets containerd v1.6, and does not get `zot`,
`skopeo` and Cloud Hypervisor ([#669][issue-669]).

### In GitHub Actions on hosted runners

The `hosted e2e` workflow runs the e2e suite directly on `ubuntu-latest` GitHub
hosted runners. It is available via `workflow_dispatch`.

The workflow prepares the runner by installing the host packages required by the
test harness, installing pinned releases of containerd, zot, Firecracker and
Cloud Hypervisor, and checking that `/dev/kvm` exists. The versions of these can
be changed with the workflow inputs. The
tests are run with `sudo` because they create loop devices, devicemapper
thinpools and a `fl-e2e-br0` bridge for the microVM TAP interfaces, write
containerd configuration under `/etc`, and manage runtime state under `/run`
and `/var/lib`.

For the overlay boot test, the workflow then:

1. Installs the pinned `mkfs.erofs` with the `erofs-utils` action of the
   repository, which builds it from source or restores it from the cache. No
   release carries the binary yet.
2. Builds `flintlock-provision` and the overlay initrd from the checkout.
3. Runs `flintlock-provision state-dir` with a sparse file of 20 GB, which
   mounts an XFS filesystem with reflinks at `/var/lib/flintlock` before the
   tests start flintlockd.
4. Starts a second `zot` on `127.0.0.1:5000`, without TLS and without
   authentication, builds the image of the initrd with `docker` and pushes it
   there. The tests get the image with the `overlay.initrd.image` flag, so they
   do not depend on a published image.

The cleanup step stops the registry, unmounts the state directory, detaches
its loop device and removes the line which `state-dir` wrote to `/etc/fstab`.

The workflow runs the tests with the `artefacts.dir` flag, and uploads what
they have saved as the `e2e-microvm-state-<attempt>` artefact of the run, for
the runs which pass and for the ones which fail. It has the console of the
guest and the stderr, the log and the config of the VMM of each of the
MicroVMs, see
[Saving the files of the MicroVMs](#saving-the-files-of-the-microvms). To get
the artefact of a run:

```bash
gh run download <run id> --name e2e-microvm-state-1
```

The tests do not save anything when they are stopped, by their timeout or by a
panic. The state directories of their MicroVMs are still there then, and the
workflow copies the files of them to the `left-behind` directory of the
artefact. It has no such directory when all of the MicroVMs were deleted.

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
- `artefacts.dir`: the directory to save the files of the state directory of
  each MicroVM to. Nothing is saved if it is not set, see
  [Saving the files of the MicroVMs](#saving-the-files-of-the-microvms).
- `providers`: comma separated list of the microvm providers to run the tests
  with. The default is `firecracker,cloudhypervisor`. The first one is the
  default provider of flintlockd, and the one which the private registry tests
  use.
- `overlay.initrd.image`: the OCI image of the initrd which boots an overlay
  root. flintlockd is started with it as `--overlay-initrd-image`. The overlay
  boot test is skipped if it is not set, see [Requirements](#requirements).

You can pass in these flags to the test like so:

```bash
./test/e2e/test.sh -level.flintlockd=9
```

To run the tests on a host which only has Firecracker:

```bash
./test/e2e/test.sh -providers firecracker
```

All the flags can be found at [`params.go`](/test/e2e/utils/params.go).

The `skip.delete` and `skip.teardown` flags leave the environment of a test
running, so the tests which come after it are skipped. Use `-run` to choose the
test to debug:

```bash
./test/e2e/test.sh -run '^TestE2EPrivateRegistry$' -skip.delete
```

### Saving the files of the MicroVMs

flintlockd removes the state directory of a MicroVM when the MicroVM is
deleted, and with it the console of the guest and the log of the VMM. With the
`artefacts.dir` flag the tests copy the files of the state directory before
they delete a MicroVM, for the tests which pass and for the ones which fail:

```bash
./test/e2e/test.sh -artefacts.dir /tmp/flintlock-e2e-artefacts
```

Use an absolute path: `go test` runs the tests in `test/e2e`, and not in the
directory which it was started from.

Each MicroVM has a directory with the name of its test, its namespace, its name
and its uid:

```text
/tmp/flintlock-e2e-artefacts/TestE2E/firecracker/firecracker-ns0/mvm0/<uid>/
    firecracker.cfg
    firecracker.log
    firecracker.metrics
    firecracker.pid
    firecracker.stderr
    firecracker.stdout
    metadata.json
```

| What | Saved |
|---|---|
| The regular files of the state directory, whatever their names are | Yes |
| Disk images (`*.img`), such as the `cloud-init.img` of a Cloud Hypervisor MicroVM | No |
| Sockets, links and directories | No |

The files are copied before the delete, so they do not have what the VMM wrote
while it was stopped. A MicroVM which flintlockd has not started to create has
no state directory, and gets no directory here.

The tests log what they have saved in a `TEST INFO:` line. A file which cannot
be saved is logged and does not fail the test.

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

[issue-669]: https://github.com/liquidmetal-dev/flintlock/issues/669
[issue-1262]: https://github.com/liquidmetal-dev/flintlock/issues/1262
[issue-1263]: https://github.com/liquidmetal-dev/flintlock/issues/1263
[zot]: https://zotregistry.dev
[skopeo]: https://github.com/containers/skopeo
[volumes-guide]: /userdocs/docs/guides/volumes.md
