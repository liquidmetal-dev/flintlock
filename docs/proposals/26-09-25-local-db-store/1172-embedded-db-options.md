# 1172. Embedded Database Options for the Local Store

* Status: informative
* Date: 2026-09-25
* Authors: @richardcase
* Issue: [#1172](https://github.com/liquidmetal-dev/flintlock/issues/1172)
* Companion to: [1172-local-db-store-requirements.md](1172-local-db-store-requirements.md)
* Design candidate: [1172-local-db-store-design.md](1172-local-db-store-design.md)

## 1. Purpose and scope

This document records the research behind the local store requirements. It
answers one question:

> Which embedded database libraries could hold flintlock's microVM records,
> and how does each measure up against the STORE requirements?

The survey was written neutrally. The decision taken on its basis is at the
end of section 8 and in section 7 of the requirements document, and will be
recorded as an ADR. Where a statement is my own judgement rather than a cited
fact it is marked *assessment*.

Every fact cites the source that owns it: the project's own documentation,
its source at a tag, or pkg.go.dev. Versions and dates were observed on
2026-09-25. Flintlock code is cited by path and line as of commit `b213882`.

## 2. Constraints from the repository

These shape the comparison more than any feature of the libraries.

* **No cgo.** Every release binary is built with `CGO_ENABLED=0` for
  `linux/amd64` and `linux/arm64`
  ([`Makefile:52-63`](../../../Makefile),
  [`.goreleaser.yaml`](../../../.goreleaser.yaml)). A driver that needs cgo
  also needs a C cross-compiler for the arm64 build (STORE-BLD-001).
* **Tiny dataset.** Records are JSON documents of a few kilobytes to a few
  tens of kilobytes; a host holds at most a few thousand of them, with a
  bounded history each. Throughput and dataset size do not discriminate
  between candidates. Query shape does: lookups by UID and by (namespace,
  name, UID), and lists by namespace or name (STORE-DATA-004).
* **One writer, one reader process.** flintlockd is the only writer. The
  separate `flintlock-metrics` binary reads records by calling the same
  `InitializePorts` ([`internal/command/metrics/serve.go:42`](../../../internal/command/metrics/serve.go)),
  so it opens whatever store flintlockd uses (STORE-CONS-007).
* **Licence.** flintlock is MPL-2.0 ([`LICENSE`](../../../LICENSE)); the
  library must be combinable with that (STORE-BLD-002).
* **Go version.** `go.mod` says `go 1.26`; the toolchain in use is 1.26.2
  (STORE-BLD-003).
* **Nothing is compiled in today.** `go.mod` has no bbolt, SQLite, Badger or
  Pebble requirement. bbolt appears in the module graph only because
  containerd v1.7.35 requires it (`go list -m all` shows
  `go.etcd.io/bbolt v1.3.10`; `go mod why -m go.etcd.io/bbolt` reports that
  the main module does not need it, and `go list -deps ./cmd/flintlockd`
  contains no bbolt or sqlite package). Adopting any candidate adds a new
  compiled dependency. The minimum-version rule would floor bbolt at v1.3.10.

## 3. Candidates

### 3.1 bbolt (`go.etcd.io/bbolt`)

* Version v1.5.0, tagged 2026-06-21. MIT.
  [pkg.go.dev](https://pkg.go.dev/go.etcd.io/bbolt),
  [releases](https://github.com/etcd-io/bbolt/releases),
  [CHANGELOG 1.5](https://github.com/etcd-io/bbolt/blob/main/CHANGELOG/CHANGELOG-1.5.md)
  (adds a data file size limit and a `NoStatistics` option).
* Pure Go. The platform layer is `syscall.Flock` and `unix.Mmap`
  ([`bolt_unix.go`](https://github.com/etcd-io/bbolt/blob/main/bolt_unix.go)).
* **Locking.** `Open` takes `flock` with `LOCK_EX`; `Options.ReadOnly` takes
  `LOCK_SH` instead. Both are tried with `LOCK_NB` in a retry loop bounded by
  `Options.Timeout`, which returns `ErrTimeout`; with no timeout the open
  waits indefinitely ([`bolt_unix.go`](https://github.com/etcd-io/bbolt/blob/main/bolt_unix.go),
  [`Options`](https://pkg.go.dev/go.etcd.io/bbolt#Options)). The README
  states that "Bolt obtains a file lock on the data file so multiple
  processes cannot open the same database at the same time", and that
  read-only mode "will block any processes from opening the database in
  read-write mode"
  ([README](https://github.com/etcd-io/bbolt/blob/main/README.md)).
  *Assessment:* a shared lock conflicts with a held exclusive lock, so a
  second process cannot open the file read-only while flintlockd holds it
  read-write. It either blocks or, with a timeout, fails.
* **Model.** One read-write transaction at a time, any number of read-only
  transactions, nested buckets, fully serialisable ACID transactions.
  Transactions and the values they return are not thread safe and byte
  slices are valid only inside the transaction (pkg.go.dev).
* **Crash safety.** Dirty pages are written and fsynced, then a new meta page
  with an incremented transaction id is written and fsynced; partially
  written data pages are ignored because the meta page that points at them
  is never written (pkg.go.dev).
* **Caveats** (README): the file never shrinks (free pages go on a freelist;
  reclaiming space needs `Compact`); a long read transaction blocks page
  reclamation; mmap can show high resident memory; the file format is
  endian-specific; corruption is possible if power fails during the initial
  creation of the file; random writes are slower than sequential ones.
* **Tooling.** `bbolt.Compact(dst, src, txMaxSize)` copies a database and
  reclaims space ([`compact.go`](https://github.com/etcd-io/bbolt/blob/main/compact.go));
  the `bbolt` CLI has `compact`, `check`, `keys`, `get` and `dump`
  ([cmd README](https://github.com/etcd-io/bbolt/blob/main/cmd/bbolt/README.md)).
* **Query capability.** Key lookups and ordered cursors with prefix seeks. No
  secondary indexes, no query language; every index is a bucket the
  application maintains.
* **containerd uses it.** containerd's metadata store is bbolt:
  [`metadata/db.go` at v1.7.35](https://github.com/containerd/containerd/blob/v1.7.35/metadata/db.go)
  imports `bolt "go.etcd.io/bbolt"`, defines `schemaVersion = "v1"` and
  `dbVersion = 3`, and `Init` "ensures the database is at the correct
  version and performs any needed migrations". The file is
  `/var/lib/containerd/io.containerd.metadata.v1.bolt/meta.db`
  ([ops.md](https://github.com/containerd/containerd/blob/v1.7.35/docs/ops.md)).
  containerd's [`go.mod`](https://github.com/containerd/containerd/blob/v1.7.35/go.mod)
  requires bbolt v1.3.10.

### 3.2 SQLite via `modernc.org/sqlite` (cgo-free transpile)

* Version v1.59.0, 2026-09-15. BSD-3-Clause; SQLite itself is public domain.
  Embeds SQLite 3.53.4. "Package sqlite is a sql/database driver using a
  CGo-free port of the C SQLite3 library." Supported targets include
  `linux/amd64` and `linux/arm64` (plus 386, arm, loong64, ppc64le, riscv64,
  s390x, and other operating systems). Driver name `sqlite`.
  [pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite),
  [README](https://gitlab.com/cznic/sqlite/-/blob/master/README.md),
  [doc.go](https://gitlab.com/cznic/sqlite/-/blob/master/doc.go).
* **WAL and pragmas.** The DSN accepts `_journal_mode=WAL`,
  `_busy_timeout`, `_txlock=deferred|immediate|exclusive`, generic
  `_pragma=` and `vfs=` (pkg.go.dev).
* **Locking.** The transpiled unix VFS uses POSIX `fcntl` locks. Open file
  description (OFD) locks are opt-in through `MODERNC_SQLITE_OFD_LOCK=1` or
  `sqlite.OFDLocking(true)`, which "must be called before the first database
  file is opened"; they stop an unrelated `os.File` close in the same process
  from stripping SQLite's locks (pkg.go.dev). *Assessment:* because it
  reproduces SQLite's own unix VFS, it interoperates with other SQLite
  processes on the same host. I did not find a sentence in its documentation
  that states multi-process WAL support explicitly; the SQLite documentation
  in section 5 is the authority.
* **Performance.** "The transpiled SQLite core runs slower than the same C
  compiled natively": 1.3x to 2.0x on CPU-bound benchmarks, comparable on
  I/O-bound ones, with the same query planner (pkg.go.dev).
* **JSON.** The JSON functions are built into SQLite by default since 3.38.0;
  the `->` and `->>` operators since 3.38.0; JSONB since 3.45.0
  ([json1](https://sqlite.org/json1.html)). *Assessment:* 3.53.4 has
  `json_extract` unless the port was built with `SQLITE_OMIT_JSON`; I did
  not inspect its build flags.
* **Pinning.** "You should use in your go.mod file the exact same version of
  modernc.org/libc as seen in the go.mod file of this repository"
  (pkg.go.dev). This is the kind of transitive pin STORE-BLD-005 asks to
  document.
* **Binary size.** Only secondary evidence: a blog post summarised in search
  results claims 15-20 MB more than the cgo driver
  ([Medium](https://medium.com/@arthurpro/your-go-binary-is-pure-go-except-for-sqlite-4c90f25cb96e),
  which refused the fetch). Unverified; STORE-BLD-006 asks for a measurement.

### 3.3 SQLite via `github.com/mattn/go-sqlite3` (cgo)

* Version v1.14.52, 2026-09-05. MIT.
  [pkg.go.dev](https://pkg.go.dev/github.com/mattn/go-sqlite3).
* "Because this is a `CGO` enabled package, you are required to set the
  environment variable `CGO_ENABLED=1` and have a `gcc` compiler present
  within your path." Cross-compiling needs a C cross toolchain
  (`CC=aarch64-linux-gnu-gcc` and so on); static binaries need
  `-ldflags "-linkmode external -extldflags -static"`. Build tags select
  features (`sqlite_json`, `sqlite_fts5`). Its advice for "database is
  locked" is `cache=shared` with `SetMaxOpenConns(1)` (pkg.go.dev).
* *Assessment:* excluded by STORE-BLD-001 as the build stands. Adopting it
  would mean enabling cgo, adding an aarch64 cross compiler to CI and
  release, and handling static linking. It is listed for completeness and
  because it is the most widely used SQLite driver for Go.

### 3.4 `github.com/ncruces/go-sqlite3` (pure Go, wasm2go)

* Version v0.35.6, 2026-09-23. MIT. `go 1.26.0` directive.
  [pkg.go.dev](https://pkg.go.dev/github.com/ncruces/go-sqlite3),
  [README at v0.35.6](https://github.com/ncruces/go-sqlite3/blob/v0.35.6/README.md),
  [go.mod at v0.35.6](https://github.com/ncruces/go-sqlite3/blob/v0.35.6/go.mod).
* It is no longer a WebAssembly runtime at run time: "It wraps a Wasm build of
  SQLite, and uses wasm2go to translate it to Go." "Go and x/sys are the
  only required dependencies." The `go.mod` requires
  `github.com/ncruces/go-sqlite3-wasm/v6` and `golang.org/x/sys` and no
  wazero (README, go.mod).
* **Platforms.** Tested on Linux amd64, arm64, 386, arm, riscv64, ppc64le,
  loong64 and s390x, plus macOS, Windows and the BSDs. Its support matrix
  gives `linux/amd64` and `linux/arm64` full file locking and full
  shared-memory WAL: "Multiple processes can access WAL databases"
  ([support matrix](https://github.com/ncruces/go-sqlite3/wiki/Support-matrix)).
* **VFS.** Pure Go. On Linux it uses OFD locks (kernel 3.15 or later) and
  `mmap` for the WAL index; alternative `sqlite3_flock` and `sqlite3_dotlk`
  build tags exist; "Concurrently accessing databases using incompatible
  VFSes will eventually corrupt data"
  ([vfs docs](https://pkg.go.dev/github.com/ncruces/go-sqlite3/vfs)).
  Goroutine-safe as long as one connection is not used concurrently; memory
  per connection is higher than C; "Performance is competitive with
  alternatives" (README).
* *Assessment:* pre-1.0 API, very active, one primary maintainer. The
  `database/sql` driver is `github.com/ncruces/go-sqlite3/driver`. It needs
  exactly the Go version flintlock already uses (STORE-BLD-003).

### 3.5 Badger and Pebble (brief)

* Badger v4.9.6, 2026-08-05, Apache-2.0, pure Go, LSM tree with a separate
  value log. Directory lock: only one process can open it
  (`WithBypassLockGuard` disables the check). Value log garbage collection is
  manual (`RunValueLogGC`).
  [pkg.go.dev](https://pkg.go.dev/github.com/dgraph-io/badger/v4),
  [README](https://github.com/dgraph-io/badger/blob/main/README.md).
* Pebble v2.1.7, 2026-08-24, BSD-3-Clause, pure Go, "focused on performance
  and internal usage by CockroachDB"; single-process directory lock,
  memtables, WAL and background compaction.
  [pkg.go.dev](https://pkg.go.dev/github.com/cockroachdb/pebble/v2).
* *Assessment:* both are single-process by design (no read-only second
  opener), run background compaction with the memory that implies, and have
  no query language. For a few thousand small records they add machinery
  with no benefit. Not considered further.

## 4. Prior art in similar daemons

| Project | Local state store | Notes |
| ------- | ----------------- | ----- |
| containerd | bbolt | Metadata store (section 3.1). Single-process daemon; namespaced nested buckets; serialisable transactions used for garbage-collection bookkeeping (*assessment* of why; no written rationale found in the 1.7 tree). |
| Podman | bbolt, then SQLite | v4.5.0 (2023-04-14): "Podman can now use a SQLite database as a backend for increased stability. The default remains … BoltDB. The database to use is selected through the `database_backend` field" ([v4.5.0 notes](https://github.com/containers/podman/releases/tag/v4.5.0)). v5.0.0: "It is no longer possible to create new BoltDB databases … Existing BoltDB databases remain usable" ([v5.0.0 notes](https://github.com/containers/podman/releases/tag/v5.0.0)). v6.0.0 (2026-06-24): "Support for BoltDB databases has been dropped. Starting Podman 6 when the BoltDB database is in use will have Podman attempt an automatic migration from BoltDB to SQLite" ([v6.0.0 notes](https://github.com/containers/podman/releases/tag/v6.0.0)). `containers.conf` `database_backend` accepts `""`, `boltdb` or `sqlite`; empty means use BoltDB if it exists, else SQLite; it recommends `podman system reset` before changing the backend of an existing deployment ([containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)). `podman system migrate` moves a BoltDB database to SQLite, leaves the legacy database in place "so no data loss should occur even on failure", and asks that other Podman processes be stopped first ([podman-system-migrate(1)](https://docs.podman.io/en/latest/markdown/podman-system-migrate.1.html)). |
| Docker Engine (moby) | bbolt | libnetwork's local store is bbolt (`local-kv.db`, bucket `libnetwork`) opened with `Timeout: time.Nanosecond` so a second opener fails "fast and loudly instead of silently introducing delays" ([datastore.go](https://github.com/moby/moby/blob/master/daemon/libnetwork/datastore/datastore.go), [boltdb.go](https://github.com/moby/moby/blob/master/daemon/libnetwork/internal/kvstore/boltdb/boltdb.go)). BuildKit's cache metadata is bbolt too ([metadata.go](https://github.com/moby/buildkit/blob/master/cache/metadata/metadata.go)). |
| kubelet | none | Checkpoint files through `pkg/kubelet/checkpointmanager`; "Sig-Node community has reached a general consensus … to avoid introducing any new checkpointing support" ([pkg.go.dev](https://pkg.go.dev/k8s.io/kubernetes/pkg/kubelet/checkpointmanager)). |
| CRI-O | none | State lives in containers/storage under `--root` and `--runroot`; exit files under `/var/run/crio/exits` ([crio(8)](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)). Rebuilt from storage on restart (*assessment*). |
| Incus | Cowsql (distributed SQLite) | "When using Incus as a single machine … effectively behaves like a regular SQLite database"; files under `/var/lib/incus/database/` ([database docs](https://linuxcontainers.org/incus/docs/main/database/)). Backup with `incus admin sql local .dump`; "there is no trivial method to restore" ([backup docs](https://linuxcontainers.org/incus/docs/main/backup/)). |

*Assessment:* the two projects closest to flintlock in shape, containerd and
Docker, chose bbolt and never needed a second process to read the file.
Podman, whose `podman` CLI processes all open the database, moved to SQLite
and cited stability. Both patterns are defensible; the deciding factor for
flintlock is STORE-CONS-007.

## 5. Cross-process access

This is where the candidates differ most, and it determines how
`flintlock-metrics` gets its records.

### 5.1 bbolt

No. The writer holds `LOCK_EX`; a read-only opener needs `LOCK_SH` and
conflicts (section 3.1). The daemon must be the only opener. A reader
process would have to obtain records from flintlockd over gRPC, read a
periodically exported copy (for example one produced with `Compact` or the
export command of STORE-MIG-012), or the daemon would have to expose read
queries itself. Docker's nanosecond timeout is worth copying so that a
double open fails immediately (STORE-CONS-006).

### 5.2 SQLite in WAL mode

Yes, on a local filesystem. In write-ahead-log mode "writers and readers can
run at the same time. However … there can only be one writer at a time". "All
processes using a database must be on the same host computer; WAL does not
work over a network filesystem." The WAL index lives in a `-shm` file that
every connection maps ([wal.html](https://sqlite.org/wal.html)). Since SQLite
3.22.0 a read-only process needs the `-shm` and `-wal` files to exist and be
readable, or write permission on the directory to create them. SQLite cannot
designate a writer; readers open with `SQLITE_OPEN_READONLY` and the
application enforces single-writer
([SQLite forum](https://sqlite.org/forum/forumpost/3fec19a873?t=h)).

Caveats:

* A read-only connection still touches `-shm` and can see a transient
  `SQLITE_BUSY` on open or close while the WAL is empty; set a
  `busy_timeout` ([note, 2026-07-26](https://hynek.me/til/sqlite-read-only-wal-locked/)).
* A permanently open reader causes "checkpoint starvation": the WAL "will
  grow without bound" (wal.html). This is why STORE-CONS-009 asks readers to
  hold transactions only per request.
* POSIX advisory locks are "buggy or even unimplemented on many NFS
  implementations" ([lockingv3](https://sqlite.org/lockingv3.html)); hence
  STORE-CONS-005.
* All processes must use lock-compatible VFS implementations. Mixing a
  dot-lock VFS with an `fcntl` one corrupts data (section 3.4). Both pure-Go
  drivers use `fcntl` or OFD locks on Linux, so a modernc writer and a
  ncruces reader, or vice versa, should interoperate (*assessment*; not
  tested).

### 5.3 Summary

| Database | Reader process while flintlockd writes | Route for `flintlock-metrics` |
| -------- | -------------------------------------- | ----------------------------- |
| bbolt | Not possible | gRPC API, or an exported copy |
| SQLite (any driver), WAL mode | Supported on a local filesystem | Direct read-only open, per-request transactions |

## 6. Migration and backup tooling worth borrowing

* **Podman's rollout**: a configuration-selected backend whose empty value
  means "use the old store if it exists, else the new one"; a one-shot
  `system migrate` that leaves the legacy database in place; an explicit
  "stop other processes first"; and removal of the old backend only two
  major versions later with automatic migration. STORE-MIG-005 and open
  question 4 follow this.
* **bbolt**: `Compact` doubles as a consistent copy and is the only way to
  shrink a file; `bbolt check` verifies integrity (STORE-OPS-002,
  STORE-OPS-003).
* **SQLite**: `.dump` (Incus), `VACUUM INTO` for an online consistent copy,
  and `PRAGMA integrity_check` (*assessment*: standard features, not verified
  in this pass against the pure-Go drivers).
* *Assessment:* a backend-neutral JSON export and import, as STORE-MIG-012
  suggests, is simpler than any of these and covers every direction,
  including content store to local store, which no database tool can do.

## 7. Comparison

| Library | Pure Go | Licence | Reader process while writer active | Query capability | Maturity and maintenance | Binary size impact |
| ------- | ------- | ------- | ---------------------------------- | ---------------- | ------------------------ | ------------------ |
| bbolt v1.5.0 | Yes | MIT | No (`flock` exclusive vs shared) | Key lookup, ordered prefix cursors; indexes are application buckets | Mature; used by etcd, containerd, moby; active under etcd-io | Small (one package plus `x/sys`) |
| modernc.org/sqlite v1.59.0 | Yes (transpiled C) | BSD-3 (SQLite public domain) | Yes (WAL, same host, local filesystem) | Full SQL, indexes, JSON functions | Mature, frequent releases; `modernc.org/libc` must be pinned | Large; 15-20 MB reported, unverified |
| mattn/go-sqlite3 v1.14.52 | No (cgo) | MIT | Yes (WAL) | Full SQL, indexes, JSON with a build tag | Very mature | Moderate C object; breaks the `CGO_ENABLED=0` arm64 build |
| ncruces/go-sqlite3 v0.35.6 | Yes (wasm2go) | MIT | Yes (WAL; full shared memory on linux amd64 and arm64) | Full SQL, indexes, JSON functions | Pre-1.0, very active, one primary maintainer; needs Go 1.26 | Moderate to large; not measured |
| Badger v4.9.6 | Yes | Apache-2.0 | No (directory lock) | Key lookup, iterators | Mature; manual value-log GC | Moderate, plus background goroutines and memory |
| Pebble v2.1.7 | Yes | BSD-3 | No (directory lock) | Key lookup, iterators | Mature, CockroachDB-focused | Moderate, plus compaction machinery |

## 8. Assessment against the requirements

Requirements every serious candidate meets: STORE-BLD-002 (licence),
STORE-BLD-003 (Go version), STORE-BLD-007 (no external daemon),
STORE-CONS-003 and STORE-CONS-004 (durability and crash safety, with the
right open options), STORE-DATA-004 (indexed lookups, provided the
application maintains index buckets in a key-value store).

Requirements that discriminate:

| Requirement | bbolt | SQLite, pure-Go driver | SQLite, cgo driver |
| ----------- | ----- | ---------------------- | ------------------ |
| STORE-BLD-001 no cgo | Meets | Meets | Fails as the build stands |
| STORE-BLD-004 maintenance, already in containerd's graph | Meets; in containerd's graph | Meets for modernc (mature); ncruces is pre-1.0 | Meets |
| STORE-BLD-005 transitive pins | None | modernc: pin `modernc.org/libc`; ncruces: pin `go-sqlite3-wasm` | None |
| STORE-BLD-006 binary size | Small | Large; must be measured | Moderate |
| STORE-CONS-006 bounded open | `Options.Timeout` | `busy_timeout` and OFD locks | `busy_timeout` |
| STORE-CONS-007 reader process | Not possible; needs the gRPC or export route | Meets (WAL, read-only open) | Meets (WAL) |
| STORE-CONS-009 reader does not starve maintenance | Not applicable (no second process); long read transactions do block page reclaim in-process | Per-request transactions needed to avoid checkpoint starvation | Same |
| STORE-DATA-003 structured, queryable fields | Only by maintaining index buckets by hand | Native columns and indexes; JSON functions for ad-hoc queries (the issue's motivation) | Same |
| STORE-DATA-007 pruning in the same transaction | Meets | Meets | Meets |
| STORE-OPS-002 online consistent copy | `Tx.WriteTo` / `Compact` | `VACUUM INTO` or the backup API | Same |
| STORE-OPS-003 integrity check | `bbolt check` | `PRAGMA integrity_check` | Same |

Deciding criteria, in the order I would weigh them:

1. **STORE-CONS-007.** If `flintlock-metrics` should keep reading the store
   directly, bbolt is out and the choice is between the two pure-Go SQLite
   drivers. If `flintlock-metrics` moves to the gRPC API (open question 2),
   bbolt becomes viable and is the smaller, better-trodden dependency.
2. **The issue's stated motivation** is queryability. Only SQLite gives
   operators and future features (snapshot records, open question 5) a query
   language and secondary indexes without application code.
3. **Dependency risk.** bbolt is already in containerd's graph and has the
   longest record. modernc is mature but carries a pinning quirk and a large
   binary. ncruces is the most modern pure-Go SQLite but is pre-1.0.
4. **Binary size** (STORE-BLD-006) should be measured before the ADR, not
   estimated.

### Decision

Taken on 2026-09-25 (requirements, section 7): **SQLite through
`modernc.org/sqlite`**. `flintlock-metrics` keeps reading the store directly
while flintlockd is stopped, which needs a second read-only process
(STORE-CONS-007) and rules out bbolt; the issue's motivation is queryability,
which only SQLite provides without application-maintained indexes; and of the
pure-Go SQLite drivers modernc is the mature one. Binary size is to be
measured in the change that adds the dependency, and the `modernc.org/libc`
pin documented in `CONTRIBUTING.md`. If the measured size is unacceptable,
`ncruces/go-sqlite3` is the fallback with the same capabilities.

## 9. References

* Issue [#1172](https://github.com/liquidmetal-dev/flintlock/issues/1172)
* bbolt: [pkg.go.dev](https://pkg.go.dev/go.etcd.io/bbolt),
  [README](https://github.com/etcd-io/bbolt/blob/main/README.md),
  [`bolt_unix.go`](https://github.com/etcd-io/bbolt/blob/main/bolt_unix.go),
  [`compact.go`](https://github.com/etcd-io/bbolt/blob/main/compact.go),
  [cmd README](https://github.com/etcd-io/bbolt/blob/main/cmd/bbolt/README.md),
  [releases](https://github.com/etcd-io/bbolt/releases)
* containerd v1.7.35: [`metadata/db.go`](https://github.com/containerd/containerd/blob/v1.7.35/metadata/db.go),
  [`docs/ops.md`](https://github.com/containerd/containerd/blob/v1.7.35/docs/ops.md),
  [`go.mod`](https://github.com/containerd/containerd/blob/v1.7.35/go.mod)
* modernc.org/sqlite: [pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite),
  [README](https://gitlab.com/cznic/sqlite/-/blob/master/README.md),
  [doc.go](https://gitlab.com/cznic/sqlite/-/blob/master/doc.go)
* mattn/go-sqlite3: [pkg.go.dev](https://pkg.go.dev/github.com/mattn/go-sqlite3)
* ncruces/go-sqlite3: [pkg.go.dev](https://pkg.go.dev/github.com/ncruces/go-sqlite3),
  [README v0.35.6](https://github.com/ncruces/go-sqlite3/blob/v0.35.6/README.md),
  [go.mod v0.35.6](https://github.com/ncruces/go-sqlite3/blob/v0.35.6/go.mod),
  [support matrix](https://github.com/ncruces/go-sqlite3/wiki/Support-matrix),
  [vfs](https://pkg.go.dev/github.com/ncruces/go-sqlite3/vfs)
* Badger: [pkg.go.dev](https://pkg.go.dev/github.com/dgraph-io/badger/v4),
  [README](https://github.com/dgraph-io/badger/blob/main/README.md);
  Pebble: [pkg.go.dev](https://pkg.go.dev/github.com/cockroachdb/pebble/v2)
* SQLite: [WAL](https://sqlite.org/wal.html),
  [file locking](https://sqlite.org/lockingv3.html),
  [JSON functions](https://sqlite.org/json1.html),
  [forum: read-only WAL access](https://sqlite.org/forum/forumpost/3fec19a873?t=h),
  [note on read-only WAL locking](https://hynek.me/til/sqlite-read-only-wal-locked/)
* Podman: [v4.5.0](https://github.com/containers/podman/releases/tag/v4.5.0),
  [v5.0.0](https://github.com/containers/podman/releases/tag/v5.0.0),
  [v6.0.0](https://github.com/containers/podman/releases/tag/v6.0.0),
  [containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md),
  [podman-system-migrate(1)](https://docs.podman.io/en/latest/markdown/podman-system-migrate.1.html)
* moby: [libnetwork datastore](https://github.com/moby/moby/blob/master/daemon/libnetwork/datastore/datastore.go),
  [boltdb kvstore](https://github.com/moby/moby/blob/master/daemon/libnetwork/internal/kvstore/boltdb/boltdb.go);
  BuildKit [cache metadata](https://github.com/moby/buildkit/blob/master/cache/metadata/metadata.go)
* kubelet [checkpointmanager](https://pkg.go.dev/k8s.io/kubernetes/pkg/kubelet/checkpointmanager);
  CRI-O [crio(8)](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)
* Incus [database](https://linuxcontainers.org/incus/docs/main/database/),
  [backup](https://linuxcontainers.org/incus/docs/main/backup/)
