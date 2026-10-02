# 1172. Local Database Backing Store Requirements

* Status: proposed
* Date: 2026-09-25
* Authors: @richardcase
* Issue: [#1172](https://github.com/liquidmetal-dev/flintlock/issues/1172)
* Companions: [1172-embedded-db-options.md](1172-embedded-db-options.md)
  (survey of candidate databases),
  [1172-local-db-store-design.md](1172-local-db-store-design.md)
  (design candidate)

## 1. Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" in this
document are to be interpreted as described in
[BCP 14](https://www.rfc-editor.org/info/bcp14)
([RFC 2119](https://www.rfc-editor.org/rfc/rfc2119),
[RFC 8174](https://www.rfc-editor.org/rfc/rfc8174)) when, and only when, they
appear in all capitals, as shown here.

Each requirement has a unique identifier of the form `STORE-<AREA>-NNN` so
that it can be referenced from designs, reviews, and tests. Identifiers are
stable: removed requirements are marked as withdrawn rather than renumbered.

Text outside the requirement tables (background, rationale, notes) is
informative.

The requirements are written against a *local store* abstraction so that they
hold for any database. The survey in
[1172-embedded-db-options.md](1172-embedded-db-options.md) compares the
candidates against them. The decisions taken on the questions this document
raised, including the choice of SQLite through the `modernc.org/sqlite`
driver, are listed in section 7 and will be recorded as an ADR.

## 2. Background

Issue [#1172](https://github.com/liquidmetal-dev/flintlock/issues/1172) asks
for a local database, such as bolt or SQLite, as an option for storing microVM
definitions instead of the containerd content store, because the content store
is hard to query.

Today every microVM record is stored as an immutable JSON blob in containerd's
content store. Each save that changes the record writes a new blob with a
higher version. Blobs are found by walking the whole content store with label
filters; there is no index. The blobs are protected by the same containerd
lease that protects the microVM's pulled images and snapshots, and they are
only removed when containerd garbage-collects them after that lease is released
during deletion. Section 4 describes this in detail.

Nothing about the content store is required for the records themselves. It was
chosen because containerd was already a dependency. The image, snapshot and
event services in containerd remain necessary regardless of where records are
kept.

### 2.1 Goals

* Let an operator choose, per host, a local embedded database as the store for
  microVM records instead of the containerd content store.
* Make records queryable by their identifying fields without scanning every
  record.
* Keep the gRPC API, the record model and the reconciliation behaviour
  unchanged whichever store is in use.
* Provide a supported migration path in both directions so that opting in is
  reversible.
* Test both stores against the same conformance suite.

### 2.2 Non-goals

* Replacing containerd for image pulling, snapshots, leases or events.
* A networked, replicated or multi-host store. The store is local to one
  flintlockd host.
* Changing the `MicroVM` gRPC API or the `models.MicroVM` record model.
* Making the local store the default in this version. The default stays
  `containerd`; section 7 gives the planned path to changing it.
* Storing anything other than microVM records in this version. The schema is
  designed to hold further record kinds (STORE-DATA-011) so that snapshot
  records (issue [#204](https://github.com/liquidmetal-dev/flintlock/issues/204),
  SNAP-OPS-005) can be added later without migrating existing data.
* Recording the database decision formally. That is the follow-up ADR
  (section 7).

## 3. Terminology

| Term | Meaning |
| ---- | ------- |
| Record | One stored microVM: its identity (namespace, name, UID), version, spec and status. The Go type is `models.MicroVM`. |
| Content store | The existing store: microVM records as blobs in containerd's content store, in the containerd namespace configured with `--containerd-ns`. |
| Local store | The new store: microVM records in an embedded database file under flintlockd's control. |
| Backend | Either the content store or the local store, selected by configuration. |
| Repository port | The `ports.MicroVMRepository` interface through which the application reads and writes records. |
| Version | The integer `Version` field of a record. It increases by one on every save that changes the record. |
| History | The set of earlier versions of a record that the store retains alongside the latest one. |
| History bound | The maximum number of versions of one record that the local store retains, written *N* below. |
| Lease | The containerd lease named `flintlock/<namespace>/<name>/<uid>` that protects the microVM's images and snapshots (and, in the content store, its record blobs) from garbage collection. |
| Reader process | A process other than flintlockd that reads records from the same host, today `flintlock-metrics`. |
| Migration | A one-shot copy of every record from one backend to the other on the same host. |

## 4. The content store today (informative)

Flintlock code is cited by path and line as of commit `b213882`.

### 4.1 Port

The repository port
([`core/ports/repositories.go:17-31`](../../../core/ports/repositories.go))
has six methods: `Save`, `Delete`, `Get`, `GetAll`, `Exists` and
`ReleaseLease`. `Get` takes `RepositoryGetOptions` with optional `Name`,
`Namespace`, `Version` and `UID` fields (lines 9-14). `Version` is only set by
tests. `GetAll` takes a `models.ListMicroVMQuery`, a `map[string]string`
([`core/models/microvm.go:104-105`](../../../core/models/microvm.go)); the
keys in use are `namespace` and `name`.

Callers:

* `Get` by UID alone:
  [`core/application/query.go:20-24`](../../../core/application/query.go)
  (`GetMicroVM`),
  [`core/application/commands.go:165-167`](../../../core/application/commands.go)
  (`DeleteMicroVM`) and
  [`internal/command/metrics/serve.go:96-98`](../../../internal/command/metrics/serve.go)
  (the metrics binary).
* `Get` by name, namespace and UID:
  [`commands.go:71-75`](../../../core/application/commands.go) (duplicate
  check on create, which tolerates `IsSpecNotFound`) and
  [`reconcile.go:27-31`](../../../core/application/reconcile.go).
* `GetAll` with `namespace` and `name`:
  [`query.go:42`](../../../core/application/query.go), feeding the gRPC
  server and the metrics binary. The controller's resync calls it with
  `{"Namespace": ""}`
  ([`infrastructure/controllers/microvm_controller.go:200-202`](../../../infrastructure/controllers/microvm_controller.go));
  because the value is empty the filter is skipped and every record is
  returned.
* `Save` on create (state `pending`), on delete (sets `DeletedAt` and state
  `deleting`, [`commands.go:178-186`](../../../core/application/commands.go)),
  on reschedule after a failed plan
  ([`reconcile.go:75`](../../../core/application/reconcile.go)) and after a
  successful plan ([`reconcile.go:154`](../../../core/application/reconcile.go)).
* `Exists` and `ReleaseLease` from the delete plan's `runtime_repo_release`
  step ([`core/steps/runtime/repo_release.go:43,62`](../../../core/steps/runtime/repo_release.go),
  added at [`core/plans/microvm_delete.go:79`](../../../core/plans/microvm_delete.go)).
* `Delete` has no production caller. Only the integration test uses it.

### 4.2 Storage

The containerd implementation is
[`infrastructure/containerd/repo.go`](../../../infrastructure/containerd/repo.go).

* Every version of a record is a separate JSON blob in the content store,
  committed with five labels: name, namespace, type (`microvm`), version and
  UID ([`repo.go:377-387`](../../../infrastructure/containerd/repo.go), label
  names in [`content.go`](../../../infrastructure/containerd/content.go)).
* Every lookup is a `Walk` of the content store with label filters. There is
  no index; each `Get`, `GetAll` and `Exists` scans every blob in the
  namespace ([`repo.go:287-336`](../../../infrastructure/containerd/repo.go)).
* `Save` reads the latest version, skips the write if neither `Spec` nor
  `Status` changed, otherwise increments `Version` and writes a new blob
  under the microVM's lease ([`repo.go:52-116`](../../../infrastructure/containerd/repo.go)).
  Old versions are never pruned. A record created with `Version: 1` comes
  back as version 2 after its first save.
* `Get` and `GetAll` pick the highest version that matches the filters; a
  version of 0 is never selected.
* `GetAll` returns records in map iteration order, which is random.
* `Delete` walks by name and namespace only, ignoring UID, and deletes every
  matching blob ([`repo.go:203-228`](../../../infrastructure/containerd/repo.go)).
* Locks are per-process, in-memory maps keyed by a string
  ([`repo.go:362-375`](../../../infrastructure/containerd/repo.go)). `Get`
  and `Exists` lock on the name; `Save`, `Delete` and `ReleaseLease` lock on
  the full ID, so the two groups do not exclude each other.

### 4.3 Leases

`Save` writes under a lease named `flintlock/<namespace>/<name>/<uid>`
([`lease.go:10-18,56-58`](../../../infrastructure/containerd/lease.go)). The
image service uses the same lease for the microVM's pulled images and
snapshots ([`image_service.go:67,126,143`](../../../infrastructure/containerd/image_service.go),
owner set at [`core/steps/runtime/kernel_mount.go:93`](../../../core/steps/runtime/kernel_mount.go)
and [`volume_mount.go:93`](../../../core/steps/runtime/volume_mount.go)).
`ReleaseLease` deletes the lease synchronously, which makes containerd
garbage-collect the record blobs, the images and the snapshots together
([`lease.go:43-53`](../../../infrastructure/containerd/lease.go)). This is
how records disappear on deletion today; `Repo.Delete` is not called.

### 4.4 Wiring and processes

`inject.InitializePorts`
([`internal/inject/wire.go:26-41`](../../../internal/inject/wire.go))
constructs the repository, the event service and the image service, each with
its own containerd client. It is called twice inside one flintlockd process,
once for the API server
([`internal/command/run/run.go:188`](../../../internal/command/run/run.go))
and once for the controllers
([`run.go:252`](../../../internal/command/run/run.go)), and once more in the
separate `flintlock-metrics` binary
([`internal/command/metrics/serve.go:42`](../../../internal/command/metrics/serve.go)).
So there are three repository instances on a host, two of them in the same
process, and the containerd event bus is what connects the API and the
controllers.

### 4.5 Configuration and build

* `--state-dir` (default `/var/lib/flintlock`,
  [`pkg/defaults/defaults.go:44`](../../../pkg/defaults/defaults.go)) is the
  root for runtime state; the providers use `<state-dir>/vm/<id>`.
* The containerd flags are `--containerd-socket`, `--containerd-kernel-ss`
  and `--containerd-ns`
  ([`internal/command/flags/flags.go:191-207`](../../../internal/command/flags/flags.go));
  the metrics binary has the first and last of these
  ([`urfave.go:22-37`](../../../internal/command/flags/urfave.go)).
* Start-up validation in `PreRunE`
  ([`run.go:44-77`](../../../internal/command/run/run.go)) checks the
  network flags, the socket directory and the default provider; nothing
  validates containerd or the state directory.
* All release binaries are built with `CGO_ENABLED=0` for `linux/amd64` and
  `linux/arm64` ([`Makefile:52-63`](../../../Makefile),
  [`.goreleaser.yaml`](../../../.goreleaser.yaml)). No embedded database
  library is compiled into any binary today.
* `flintlockd.service` has `Requires=containerd.service`
  ([`flintlockd.service:18`](../../../flintlockd.service)).

### 4.6 Tests

`infrastructure/containerd/repo_test.go` exercises `Exists`, `Save` version
increments, `Get` latest, `Get` by version, `GetAll` by namespace and
unfiltered, `Delete` and `Get` after delete, and the no-change save skip. The
tests run only when `CTR_SOCK_PATH` is set and need root
([`Makefile:120`](../../../Makefile), [`CONTRIBUTING.md:334-341`](../../../CONTRIBUTING.md)).

## 5. Requirements

### 5.1 Backend selection and configuration (STORE-CFG)

| ID | Requirement |
| -- | ----------- |
| STORE-CFG-001 | The backend MUST be selectable per host through flintlockd configuration (a flag, with the same option available in the config file), with the values `containerd` and one value per supported local store. |
| STORE-CFG-002 | The default backend MUST be `containerd`, so that existing hosts are unaffected by upgrading. |
| STORE-CFG-003 | The same backend option MUST be accepted by `flintlock-metrics`, so that a reader process can be configured to read from the same store as flintlockd (see STORE-CONS-007). |
| STORE-CFG-004 | The local store's file path MUST default to a location under `--state-dir` and MUST be overridable. The parent directory MUST be created if missing, with the existing `DataDirPerm` mode. |
| STORE-CFG-005 | The history bound *N* MUST be configurable, with a minimum of 1 (latest version only). The default MUST be 10. |
| STORE-CFG-006 | flintlockd MUST fail to start with a clear error if the configured backend cannot be opened. It MUST NOT fall back to another backend. |
| STORE-CFG-007 | The store configuration MUST be validated in flintlockd's `PreRunE` alongside the existing checks: the backend name is known, the path is absolute, and the history bound is at least 1. |
| STORE-CFG-008 | The containerd socket and namespace MUST remain required whichever backend is selected, because the image, snapshot and event services still use containerd. |
| STORE-CFG-009 | At start-up flintlockd SHOULD warn when the selected backend is the local store, the store file did not exist before this start, and the content store still holds microVM records in the configured containerd namespace, because that usually means a migration was skipped. It MUST NOT refuse to start on this condition. |
| STORE-CFG-010 | flintlockd MUST log the selected backend, and for the local store its path and schema version, at start-up. |
| STORE-CFG-011 | `ServerInfo` MAY report the active backend. |

### 5.2 Repository port (STORE-PORT)

The port is the contract that both backends implement. Where the content
store's behaviour today is an accident of its implementation rather than a
requirement of its callers (random list order, `Delete` by name only), these
requirements fix the behaviour rather than preserve it.

| ID | Requirement |
| -- | ----------- |
| STORE-PORT-001 | The application (`core/`) MUST depend only on `ports.MicroVMRepository` and related ports. No backend-specific type, error or import MUST appear outside `infrastructure/` and the wiring in `internal/inject`. |
| STORE-PORT-002 | `Save` MUST be atomic per record: after `Save` returns, either the whole new version is stored or nothing changed. |
| STORE-PORT-003 | `Save` MUST NOT write when the incoming record's `Spec` and `Status` are equal to the latest stored version's, and MUST return the stored record in that case. This preserves the content store's behaviour, on which the reconciler's idempotence relies. |
| STORE-PORT-004 | On every save that changes a record, `Save` MUST increment `Version` by exactly one relative to the latest stored version, and the returned record MUST carry the new version. |
| STORE-PORT-005 | `Save` of a record with a `Version` that is neither 0 nor equal to the latest stored version SHOULD fail with a conflict error rather than overwrite, so that a caller working from a stale read cannot silently discard another caller's change. A `Version` of 0 or an absent record means "first save". |
| STORE-PORT-006 | `Get` MUST support lookup by UID alone and by the combination of name, namespace and UID. It MUST return an error satisfying `errors.IsSpecNotFound` when no record matches. |
| STORE-PORT-007 | `Get` with `Version` set MUST return that version if it is retained in the history and an error satisfying `IsSpecNotFound` otherwise. |
| STORE-PORT-008 | `Get` without `Version` MUST return the latest version. |
| STORE-PORT-009 | `GetAll` MUST filter by `namespace` and `name` when they are present with non-empty values, MUST ignore keys with empty values, and MUST return the latest version of each matching record only. |
| STORE-PORT-010 | `GetAll` MUST return records in a deterministic order: ascending by namespace, then name, then UID. |
| STORE-PORT-011 | `Exists` MUST report whether a record with the given namespace, name and UID has a latest version in the store. |
| STORE-PORT-012 | `Delete` MUST remove the latest version and every retained history version of the record identified by namespace, name and UID. Deleting a record that does not exist MUST succeed. |
| STORE-PORT-013 | `ReleaseLease` MUST be removed from `MicroVMRepository`. Releasing the containerd lease MUST be exposed through a port owned by the containerd integration (a new lease port or a method on `ImageService`), because the lease protects images and snapshots, which stay in containerd whichever backend holds the records. |
| STORE-PORT-014 | The delete plan MUST release the containerd lease and then call `Delete` on the repository, in that order, and the record delete MUST be the final step of the plan so that a plan that fails part-way is retried from a record that still exists. |
| STORE-PORT-015 | The content store backend MUST implement the same contract, including STORE-PORT-010, STORE-PORT-012 (delete by UID as well as name and namespace) and STORE-PORT-014. Its behaviour MAY differ only where a requirement says so explicitly. |
| STORE-PORT-016 | Both backends MUST pass the same conformance suite (STORE-TEST-001). |

### 5.3 Data model (STORE-DATA)

| ID | Requirement |
| -- | ----------- |
| STORE-DATA-001 | A record's identity MUST be the triple (namespace, name, UID). The UID MUST be unique within its record kind on its own, because callers look records up by UID alone. |
| STORE-DATA-002 | The local store MUST hold each version of a record as the `encoding/json` encoding of `models.MicroVM`, byte-for-byte the same encoding the content store writes today, so that migration is a copy and the JSON stays the single definition of the record format. |
| STORE-DATA-003 | The local store MAY additionally hold the identifying and status fields (namespace, name, UID, version, state, `CreatedAt`, `UpdatedAt`, `DeletedAt`) in indexed or structured form for querying. Where it does, the JSON MUST remain authoritative and the structured copy MUST be derived from it on every write. |
| STORE-DATA-004 | Lookups by UID, by (namespace, name, UID) and lists filtered by namespace or name MUST be served without reading the JSON of records that do not match. |
| STORE-DATA-005 | The local store MUST record a schema version. It MUST refuse to open a store whose major schema version is newer than the one it supports, with an error that names both versions. |
| STORE-DATA-006 | When the local store opens a store with an older, supported schema version, it MUST upgrade it in place before serving requests, or refuse to open with an error that names the command that performs the upgrade. Either way the store MUST NOT be left in a state that neither version can open. |
| STORE-DATA-007 | The local store MUST retain the latest version of each record and up to *N*-1 earlier versions (the history bound). Pruning of versions beyond the bound MUST happen in the same transaction as the save that makes them surplus. |
| STORE-DATA-008 | Version numbers MUST continue to increase after pruning. A version number MUST never be reused for the same record. |
| STORE-DATA-009 | Records contain the microVM's `metadata` map, which commonly holds secrets such as cloud-init user data. The local store file MUST be created with mode 0600, owned by the user flintlockd runs as. Any auxiliary files the database creates alongside it MUST have the same protection. |
| STORE-DATA-010 | A record written by an older flintlock version, whose JSON lacks fields that were added later, MUST be readable; missing fields take their zero values as they do today when the content store's blobs are decoded. |
| STORE-DATA-011 | The local store's schema MUST partition records by kind, with `microvm` as the only kind in this version. Adding a new kind (for example snapshot records from issue #204) MUST NOT require migrating or rewriting existing records, and kinds MUST NOT share an identifier space. |

### 5.4 Concurrency and durability (STORE-CONS)

| ID | Requirement |
| -- | ----------- |
| STORE-CONS-001 | Within one flintlockd process there MUST be exactly one open handle to the local store, shared by the API server and the controllers. The store MUST NOT be opened once per `InitializePorts` call. |
| STORE-CONS-002 | Concurrent `Save` calls for the same record MUST be serialised by the store, not only by in-memory locks, so that STORE-PORT-002 and STORE-PORT-004 hold even when the API server and the controller save the same record at the same time. |
| STORE-CONS-003 | A `Save` MUST be durable when it returns: the data MUST have been flushed to stable storage (fsync or the database's equivalent synchronous commit), because the reconciler relies on the stored state after a restart. |
| STORE-CONS-004 | A crash or power loss during a write MUST NOT corrupt the store or lose any version whose `Save` had already returned. The store MUST open normally after an unclean shutdown without manual repair. |
| STORE-CONS-005 | The local store file MUST be on a local filesystem. Network filesystems are NOT RECOMMENDED and MUST be documented as unsupported. |
| STORE-CONS-006 | Opening the local store MUST fail with a clear error within a bounded time if another process holds it in a way that prevents opening. It MUST NOT block indefinitely. |
| STORE-CONS-007 | A reader process on the same host MUST be able to open the store read-only and read current records while flintlockd has it open for writing, without blocking flintlockd's writes and without being blocked by them beyond a bounded wait. `flintlock-metrics` MUST keep working while flintlockd is stopped, as it does today, so the reader MUST NOT depend on the flintlockd API. A reader process MUST NOT be able to prevent flintlockd from opening its store. |
| STORE-CONS-008 | A reader process MUST open the store in read-only mode and MUST NOT be able to modify it. |
| STORE-CONS-009 | A long-lived reader MUST NOT cause unbounded growth of the store or its auxiliary files. Where the database can starve its own compaction or checkpointing under a permanently open reader, the reader MUST hold its transactions only for the duration of each request. |
| STORE-CONS-010 | Closing flintlockd MUST close the store cleanly, after in-flight saves have completed. |

### 5.5 Lifecycle and containerd interaction (STORE-LIFE)

| ID | Requirement |
| -- | ----------- |
| STORE-LIFE-001 | Image pulls, snapshots and their leases MUST continue to be owned by containerd under the existing lease name `flintlock/<namespace>/<name>/<uid>`, whichever backend holds the records. |
| STORE-LIFE-002 | Deleting a microVM MUST release its containerd lease and MUST delete its record from the selected backend. If either step fails, the plan MUST fail and be retried by the reconciler, which is possible only if the record still exists (STORE-PORT-014). |
| STORE-LIFE-003 | Marking a microVM for deletion (`DeletedAt` set, state `deleting`) MUST remain an ordinary `Save`, so that a deletion in progress survives a restart. |
| STORE-LIFE-004 | The controller's resync MUST be able to list every record with an empty or absent filter. |
| STORE-LIFE-005 | Events MUST continue to be published and consumed through the containerd event service. The local store MUST NOT publish events and MUST NOT be used as a substitute channel between the API server and the controllers. |
| STORE-LIFE-006 | Switching backends MUST NOT change any microVM's UID, name, namespace, version or state, and a microVM created under one backend MUST be reconcilable after a migration to the other. |

### 5.6 Migration (STORE-MIG)

Migration follows the pattern Podman used when it moved from BoltDB to SQLite:
an opt-in backend, a one-shot command that copies the data and leaves the
source in place, and a warning to stop other processes first (see the survey,
section 6).

| ID | Requirement |
| -- | ----------- |
| STORE-MIG-001 | A `flintlockd store` subcommand MUST copy every microVM record from one backend to the other on the same host. It MUST support both directions: content store to local store and local store to content store. |
| STORE-MIG-002 | The migration command MUST refuse to run while flintlockd or any other writer has the target or source store open. Where a backend provides no lock to detect this, the command MUST require an explicit flag acknowledging that flintlockd is stopped. |
| STORE-MIG-003 | The migration command MUST support a dry run that reports what it would copy without writing. |
| STORE-MIG-004 | The migration command MUST be idempotent: running it twice MUST leave the target identical to running it once, and it MUST NOT create duplicate records or extra versions. |
| STORE-MIG-005 | The migration command MUST NOT delete or modify the source by default. Deleting the source MAY be offered as a separate, explicit option, and MUST NOT release any containerd lease (STORE-LIFE-001). |
| STORE-MIG-006 | For every record the migration MUST preserve namespace, name, UID, the version number of every copied version, and the JSON of the spec and status byte-for-byte (STORE-DATA-002). |
| STORE-MIG-007 | When the target is the local store and the source holds more versions of a record than the history bound, the migration MUST copy the latest *N* versions and report the number dropped. |
| STORE-MIG-008 | When the target is the content store, the migration MUST write each version under the microVM's lease exactly as `Save` does, so that the containerd backend can later find and garbage-collect it. |
| STORE-MIG-009 | The migration MUST verify each record by reading it back from the target and comparing it with the source before reporting success. |
| STORE-MIG-010 | The migration MUST print a summary (records copied, versions copied, versions dropped by the bound, records skipped) and MUST exit non-zero if any record could not be copied and verified. |
| STORE-MIG-011 | The migration MUST accept records written by older flintlock versions (STORE-DATA-010). |
| STORE-MIG-012 | The migration SHOULD be implemented as a backend-neutral export to a file and import from a file, with the direct migration built from the two, so that operators can also back up, inspect and restore records with ordinary tools. |

### 5.7 Build, dependencies and portability (STORE-BLD)

| ID | Requirement |
| -- | ----------- |
| STORE-BLD-001 | The local store MUST build with `CGO_ENABLED=0` on `linux/amd64` and `linux/arm64`, as every flintlock binary does today. A database driver that needs cgo MUST NOT be adopted without a separate decision to change the build. |
| STORE-BLD-002 | The database library's licence MUST be an OSI-approved permissive or weak-copyleft licence that can be combined with flintlock's MPL-2.0; MIT, BSD, Apache-2.0 and MPL-2.0 qualify. |
| STORE-BLD-003 | The library MUST NOT require a Go version newer than the `go` directive in `go.mod`. |
| STORE-BLD-004 | The library SHOULD have a maintenance record comparable to containerd's dependencies: a stable release line, regular releases and more than one active maintainer. Libraries already in containerd's dependency graph are preferred, other things being equal. |
| STORE-BLD-005 | Any transitive dependency that must be pinned to a matching version (for example a driver's runtime library) MUST be documented in `CONTRIBUTING.md`. |
| STORE-BLD-006 | The change in binary size for `flintlockd` and `flintlock-metrics` SHOULD be measured and reported in the pull request that adds the dependency. |
| STORE-BLD-007 | The local store MUST NOT require a separate daemon, service or external binary at runtime. |

### 5.8 Operations (STORE-OPS)

| ID | Requirement |
| -- | ----------- |
| STORE-OPS-001 | It MUST be possible to back up the local store by copying its file (and any auxiliary files) while flintlockd is stopped, and to restore it by copying them back. |
| STORE-OPS-002 | An online, consistent copy of the local store SHOULD be possible while flintlockd runs, using the database's own copy mechanism, and SHOULD be exposed as a `flintlockd store` subcommand. |
| STORE-OPS-003 | An integrity check of the local store SHOULD be exposed as a `flintlockd store` subcommand that can run while flintlockd is stopped. |
| STORE-OPS-004 | flintlockd SHOULD expose metrics for the store: record count, file size, and latency of `Save`, `Get` and `GetAll`. |
| STORE-OPS-005 | Errors from the local store MUST identify the store path and the operation, so that the troubleshooting documentation can point at them. |
| STORE-OPS-006 | The user documentation MUST be updated: the containerd page no longer says containerd stores microVM metadata unconditionally; the service options page documents the new flags; the troubleshooting page for `failed to reconcile vmid` gets a local-store recipe alongside the `ctr content` one; `config.yaml.example` shows the options; `CONTRIBUTING.md` describes how to run the store tests. |
| STORE-OPS-007 | The `flintlock-provision` command MAY gain an option to configure the backend; it is not required for this version. |

### 5.9 Testing (STORE-TEST)

| ID | Requirement |
| -- | ----------- |
| STORE-TEST-001 | A backend-agnostic conformance suite for `MicroVMRepository` MUST exist, covering at least what `repo_test.go` covers today (`Exists`, version increment on `Save`, no-change skip, `Get` latest, `Get` by version, `GetAll` filtered and unfiltered, `Delete`, `Get` after delete) plus deterministic list order, history bound pruning, version conflict, and `Delete` of a missing record. Every backend MUST run it. |
| STORE-TEST-002 | The local store's tests MUST run as part of `make test` on a developer machine without containerd, KVM, root or any environment variable, using a temporary directory. |
| STORE-TEST-003 | Concurrency tests MUST cover parallel saves to the same record and to different records from multiple goroutines through one handle, and a reader in a second process (or a second read-only handle where the database allows it) while writes are in progress. |
| STORE-TEST-004 | A crash-safety test SHOULD exist: interrupt a process during a write and verify that the store opens and holds every version whose save returned. |
| STORE-TEST-005 | The migration MUST have a round-trip test (content store to local store to content store) in the containerd integration suite, asserting STORE-MIG-006. |
| STORE-TEST-006 | Mocks for any changed port MUST be regenerated with `make generate` in the same change. |

## 6. Known limitations and risks (informative)

* **Two possible sources of truth on a host.** Until a migration runs, a host
  switched to the local store has an empty store and stale records in
  containerd. STORE-CFG-009 warns; it cannot prevent operator error.
* **containerd stays a hard dependency.** The local store removes the content
  store from the record path but not containerd from the daemon: images,
  snapshots, leases and the event bus still need it (STORE-CFG-008).
* **Lease release must move.** Today record removal is a side effect of
  releasing the lease. With a separate record store the delete plan needs an
  explicit record delete (STORE-PORT-013, STORE-PORT-014), which also changes
  the containerd backend.
* **Reader process access constrains the database.** `flintlock-metrics`
  must keep reading the store directly while flintlockd is stopped
  (STORE-CONS-007). A key-value store with an exclusive file lock cannot
  support that, which is one of the two reasons the choice fell on SQLite in
  write-ahead-log mode (section 7).
* **Bounded history discards an audit trail** that the content store kept
  indefinitely. Operators who rely on old versions need the export command
  (STORE-MIG-012) before switching.
* **Shared handle refactor.** `InitializePorts` is called twice per process;
  satisfying STORE-CONS-001 needs a change to how the store is constructed and
  passed in, which touches the wiring for both backends.
* **Pure-Go database drivers** are slower than C for CPU-bound work and can
  add tens of megabytes to the binary. At flintlock's scale (thousands of
  small records) the speed is unlikely to matter; the size is measured under
  STORE-BLD-006.

## 7. Decisions (informative)

The first draft of this document raised seven open questions. They were
decided on 2026-09-25 as follows; the requirements above already reflect
them. Decision 7 will be recorded as an ADR.

1. **Default history bound.** *N* = 10 (STORE-CFG-005). It covers a create,
   the reconcile updates that follow, retries and a delete, at roughly ten
   times one record's size per microVM.
2. **`flintlock-metrics` reads the store directly**, opening it read-only
   (STORE-CONS-007, STORE-CONS-008). It works today while flintlockd is
   stopped, because Firecracker metrics are read from files under the state
   directory, and that independence is kept. Routing it through the gRPC API
   was rejected because it would tie metrics to flintlockd being up and add
   endpoint and TLS flags to the metrics binary.
3. **Migration and maintenance commands live in `flintlockd`** as
   `flintlockd store <export|import|migrate|check|backup>` (STORE-MIG-001,
   STORE-OPS-002, STORE-OPS-003). flintlockd already links both backends and
   their configuration; `flintlock-provision` does not, and a new binary is
   not justified.
4. **Deprecation path for the content store** follows Podman's three steps:
   this version is opt-in with `containerd` as the default (STORE-CFG-002);
   a later release makes the local store the default for new hosts while
   hosts that already hold records in the content store keep using it and
   are warned at start-up; a later release still removes the content store
   backend and migrates automatically at start-up. Each step is its own
   change with its own release note.
5. **Snapshot records will share the store.** The schema is partitioned by
   record kind from the start (STORE-DATA-011) so that the snapshot and
   restore work (issue #204, SNAP-OPS-005) adds a kind rather than a
   migration. Only the `microvm` kind exists in this version.
6. **Flag names** are `--store-backend`, `--store-path` and
   `--store-history`, matching the "store" vocabulary used throughout and
   the `flintlockd store` command.
7. **Database: SQLite through the `modernc.org/sqlite` driver.** It is the
   only family that satisfies decision 2 (a second read-only process, which
   bbolt's exclusive file lock rules out) and the issue's motivation
   (queries and indexes without application code). Among the pure-Go SQLite
   drivers it is the mature one; `ncruces/go-sqlite3` is pre-1.0 and the
   cgo driver is excluded by STORE-BLD-001. The costs are a large binary,
   to be measured under STORE-BLD-006, and the `modernc.org/libc` pin to be
   documented under STORE-BLD-005. See the survey, section 8.

No open questions remain.

## 8. References

* Issue [#1172](https://github.com/liquidmetal-dev/flintlock/issues/1172)
* [1172-embedded-db-options.md](1172-embedded-db-options.md): survey of
  candidate databases against these requirements
* [1172-local-db-store-design.md](1172-local-db-store-design.md): design
  candidate
* Snapshot and restore requirements, SNAP-OPS-005 (snapshot records must
  persist across restarts):
  [0204-snapshot-restore-requirements.md](../26-09-25-snapshot-restore/0204-snapshot-restore-requirements.md)
* `modernc.org/sqlite`: [pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite)
* Podman `database_backend` and `podman system migrate`:
  [containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md),
  [podman-system-migrate(1)](https://docs.podman.io/en/latest/markdown/podman-system-migrate.1.html)
* [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and
  [RFC 8174](https://www.rfc-editor.org/rfc/rfc8174)
