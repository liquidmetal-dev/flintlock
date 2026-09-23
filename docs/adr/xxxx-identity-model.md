# Identity Model for flintlock Hosts, Clients and microVMs

* Status: proposed
* Date: 2026-09-23
* Authors: @richardcase
* Deciders: @richardcase @steve-fraser
* ADR Discussion: _to be added when the discussion is opened_

## Context

flintlock offers three authentication postures — none, a static shared token,
and mTLS — and all three discard who is calling.

`pkg/auth/basic.go:37-38` writes `Authenticated` and `AuthMethod` into the
request context, and nothing in the repository ever reads them; the only
reference outside `pkg/auth/` is a single import in
`internal/command/run/run.go`. `pkg/auth/tls.go:43-45` sets
`tls.RequireAndVerifyClientCert`, which proves the peer certificate chains to
the configured CA, and then discards the certificate — there is no
`peer.FromContext`, `credentials.TLSInfo` or `VerifiedChains` call anywhere in
the tree. So `hack/scripts/gen_local_certs.sh` issues client certificates with
`CN="Liquid Metal Client 1"`, and that CN has no runtime meaning at all.

Authentication is a pure gate: it either lets an RPC through or rejects it, and
nothing downstream ever varies by caller. What follows from that:

- Any holder of any CA-issued certificate can get, list and delete any microVM
  in any namespace. `namespace` is caller-supplied and unconstrained
  (`infrastructure/grpc/server.go:158-166`).
- `MicroVMExec` and `MicroVMSSHProxy` — shell-level access into guests — sit
  behind the identical single global gate as a read-only `ServerInfo` call.
- `MicroVMSpec` records `CreatedAt`, `UpdatedAt` and `DeletedAt`, and no
  creator.
- `SECURITY.md` puts "any issue reachable by an authenticated client that goes
  beyond what the API is meant to allow" in scope. Today an authenticated
  client is meant to be allowed everything, so that sentence has nothing to
  bite on.

This ADR settles the identity model so that the implementation issues under #7
can proceed. It covers naming, where identities come from, how a caller becomes
a principal, and how requests are authorized. It changes the API surface, the
domain model, the configuration and the security posture, which is why it is an
ADR rather than a design note on an issue.

### Why SPIFFE

We adopt SPIFFE as the identity *vocabulary* and make SPIRE an optional
*source*.

The flintlock-specific argument is that flintlockd is dialled by IP on bare
metal — the documented usage is `hammertime create -a 192.168.1.66:9090`.
Conventional TLS identity is hostname verification, which fits badly with
re-imaged hosts on DHCP and IP SANs. SPIFFE deliberately decouples identity
from network location, which is the shape of this problem.

What it buys us:

1. **Naming.** A `spiffe://<trust-domain>/...` URI carried in the certificate's
   URI SAN. That field is already inside the certificate flintlock already
   verifies, so extracting it is a few lines rather than a subsystem.
2. **Mutual host identity.** A client matches the server's SPIFFE ID rather
   than a DNS name.
3. **A rotation interface.** We need one regardless: `pkg/auth/tls.go:19` loads
   the keypair once at process start, so flintlockd cannot pick up a renewed
   certificate without a restart.
4. **Guest identity over a delivery path that already exists.**
   `MicroVMSpec.metadata` reaches the guest via MMDS on Firecracker or the
   cloud-init drive on Cloud Hypervisor, and `addInstanceData`
   (`core/application/commands.go:199-235`) is precedent for flintlock
   injecting its own values into it.
5. **`VMID` is already a SPIFFE-shaped name.** `core/models/vmid.go` is
   `{name, namespace, uid}`.

### What SPIFFE does not give us

SPIFFE's real security value is SPIRE's *attestation* — proving a workload is
what it claims before issuing it a credential. SPIFFE-compatible **without**
SPIRE gives us naming, mutual verification and rotation plumbing, but trust
still reduces to "whoever holds the key file". That is no worse than today's
mTLS, and it means flintlock slots into a SPIRE deployment later with no API
change, but it is not attestation and we should not present it as such.

SPIFFE also provides no authorization. SPIFFE IDs are names; policy is still
ours to write.

## Decision

### Identity

**1. SPIFFE is the vocabulary; SPIRE is an optional source.** If a Workload API
socket is configured via `--spiffe-workload-api-socket`, use it. Otherwise fall
back to credentials on disk, as today.

**2. The naming scheme is:**

```
spiffe://<trust-domain>/flintlock/host/<host-id>
spiffe://<trust-domain>/flintlock/client/<name>
spiffe://<trust-domain>/flintlock/microvm/<namespace>/<name>/<uid>
```

`VMID.String()` is `namespace/name/uid` (`core/models/vmid.go:81`), so the
third form is exactly `spiffe://<trust-domain>/flintlock/microvm/` +
`VMID.String()`.

The uid is included deliberately. Without it, a microVM deleted and recreated
with the same name in the same namespace has the same identity as its
predecessor and inherits whatever authority that identity was granted. Note
that `NewVMID` does not require a uid (`core/models/vmid.go:30-46`) — it is
stamped later, at `core/application/commands.go:63-68` — so a microVM identity
only exists from that point onwards.

**3. The scheme is normative for what flintlock mints, and descriptive for
clients.** flintlock generates and cross-checks the `host` and `microvm` forms.
It does not issue client SVIDs — SPIRE or your PKI does — so the `client` form
is what we recommend and what our own tooling generates, not a gate. Any
well-formed SPIFFE ID presented by a verified peer is accepted as a principal.
Rejecting foreign path shapes would buy nothing, because the policy engine is
what decides, and it would make flintlock hostile to any existing SPIFFE
deployment with its own conventions.

**4. `name` and `namespace` get a constrained character set,** enforced as
model validation on `VMID`. These segments become URI path components, so a `/`
in either one produces a different identity than the caller asked for. We
already have both the machinery — ADR 0003 chose model-level validation with
`go-playground/validator` — and the precedent: `GuestDeviceName` is tagged
`validate:"required,excludesall=/@,guestDeviceName"` at
`core/models/network.go:12`. This also fixes a latent bug that predates this
ADR: a microVM named `a/b` produces a four-part `VMID.String()` which
`splitVMIDFromString` then rejects (`core/models/vmid.go:113-130`).

**5. The trust domain is configured by `--trust-domain`,** config-file key
`trust-domain`, environment variable `FLINTLOCKD_TRUST_DOMAIN`. When it is
unset, flintlock mints no SPIFFE IDs and the principal model falls back to its
non-SPIFFE kinds; everything else keeps working. We do not ship a default trust
domain. A shared default value is precisely the collision that trust domains
exist to prevent.

**6. A peer from a different trust domain is accepted as a principal,** and
policy decides what it may do. Trust-domain separation is enforced by which CA
you configure, not by a string comparison we perform after chain validation has
already succeeded. SPIFFE federation is out of scope.

**7. The host ID comes from `--host-id`, defaulting to `os.Hostname()`, and is
cross-checked against the server certificate's URI SAN. A mismatch refuses to
start.** flintlock does not mint its own server certificate; the operator
supplies it. So the flag states the operator's intent and the certificate is
the credential, and disagreement between them is a misconfiguration worth
failing on — the same class of problem as a client CA that is configured but
never enforced. If a trust domain is configured but the server certificate
carries no SPIFFE URI SAN, that is a warning and no host identity exists.

**8. A caller becomes one of five principal kinds.** These are part of the
public surface, because policy is written against how they render:

| Kind | Source | Rendering |
|---|---|---|
| `spiffe` | verified URI SAN | `spiffe://<trust-domain>/...` |
| `x509` | verified chain, no SPIFFE URI | `x509://<RFC 2253 subject DN>` |
| `token` | `--basic-auth-token` | `token://static` |
| `anonymous` | no credential | `anonymous` |
| `system` | flintlockd itself | the host's own SPIFFE ID |

`x509` renders the full subject DN rather than the CN alone, because a CN is
ambiguous across CAs and CN-as-identity has been deprecated for years, and
rather than a public-key fingerprint, because a fingerprint changes on every
renewal and would defeat the rotation story this ADR is partly about.

`system` exists so that reconciler activity is attributable. The reconciler has
no caller and is exempt from policy, but a microVM torn down or recreated by
reconciliation must not leave a hole in the decision log, because the natural
reading of a hole is that nobody did it.

**9. microVM identities are named here; issuing them is deferred to its own
ADR.** This ADR fixes the name form. Whether flintlockd becomes a signing
authority, or brokers credentials from SPIRE, is a materially different trust
decision and gets its own discussion.

The motivating use case for that ADR is worth naming now, so that it starts
from a concrete problem: flintlockd reaches into a running guest over vsock for
`MicroVMExec` and `MicroVMSSHProxy` (`infrastructure/grpc/exec_server.go:68`)
with no authentication in either direction. flintlockd trusts the socket
because it created it; the guest agent trusts whatever connects. Today that
channel's security rests entirely on host-local socket permissions. Giving a
guest an identity is what would let either end verify the other.

### Authorization

**10. Authorization delegates to Cedar (`cedar-policy/cedar-go`) behind a
`ports.Authorizer` seam.** We chose Cedar over OPA/Rego on two grounds: the
decision shape here is principal × action × resource, which is Cedar's native
model rather than something expressed in a general-purpose policy language; and
the dependency is materially lighter, which matters for a daemon shipped to
bare metal. The seam is the durable part of this decision — the engine sits
behind `ports.Authorizer` like every other adapter in the tree, and can be
replaced without touching the domain.

**11. Entity types and action names are fixed here; the attribute set is not.**
Actions are service-qualified short names with no API version:

```
MicroVM::Create        MicroVM::Delete       MicroVM::Get
MicroVM::List          MicroVM::ServerInfo
MicroVMExec::Exec      MicroVMSSHProxy::Proxy
```

Leaving `v1alpha1` out is deliberate: policies written today should survive the
bump to `v1`. Which attributes a resource exposes to policy will settle in
implementation, because attributes are additive and action names are not.

**12. Enforcement happens in two stages:** a coarse method-level check in an
interceptor, and a resource-level check inside `core/application` after the
repository fetch.

An interceptor alone is not sufficient, and this supersedes the design
sketched on #1233. `GetMicroVM` and `DeleteMicroVM` take a bare uid
(`core/ports/usecases.go:14,20`), and the namespace and name are only known
after the repository lookup (`core/application/commands.go:165`,
`core/application/query.go:24`). `MicroVMExec` and `MicroVMSSHProxy` are the
same. An interceptor sees the method and the uid and nothing else, so it cannot
express "only in your own namespace" for exactly the calls that most need it.

Three consequences of that placement:

- Not-authorized and not-found are returned as the same error. Otherwise the
  authorization check becomes an existence oracle for every uid.
- `ListMicroVMs` and `ListMicroVMsStream` (`infrastructure/grpc/server.go:146`,
  `:191`) filter their results rather than returning allow or deny.
- `ReconcileMicroVM` (`core/application/reconcile.go:22`) is exempt from
  policy, because it has no caller, while still being attributed as `system`.

**13. `namespace` is the tenancy boundary.** Today it is a naming and grouping
key with no access-control semantics. From this ADR onwards a policy may bind a
principal to a namespace, and namespace becomes security-relevant. The default
namespace is an ordinary namespace with no special treatment: policy must grant
it like any other. We are not making `namespace` a required API field as part
of this decision.

**14. Authorization has three modes,** selected by `--authz-mode`:

| Mode | Behaviour |
|---|---|
| `off` | No authorization. Logs a warning at startup. |
| `audit` | Evaluate and log, change nothing. |
| `enforce` | Evaluate and enforce. |

`--authz-policy` names either a file or a directory of policies, resolved by
stat. The default is `off` when no policy is configured, and **`enforce` when a
policy is configured and no mode is set**. That asymmetry is the point: it is
the only default under which nobody configures a policy and silently gets no
protection from it, and under which no existing deployment changes behaviour on
upgrade.

`audit` is not a dry run of `enforce` for list operations. A list returns
unfiltered in `audit` while logging what would have been removed, because
`audit` means "change nothing and tell me what would happen".

Policy reload is out of scope for the first implementation; a restart picks up
changes. Re-parsing and atomically swapping a policy set is its own design
problem, and folding it into the credential-reload work would have this ADR
promise a design neither piece of work has done.

**15. An authorization configuration that cannot be honoured is a startup
failure, never a silent downgrade.** Three applications of that principle:

- A policy that fails to compile refuses to start.
- `enforce` with no policy configured refuses to start.
- A policy configured with TLS enabled and `--tls-client-validate` off refuses
  to start, unless the policy refers only to token and anonymous principals.
  Without client validation every caller is anonymous, so a policy naming
  certificate-derived principals denies everything for a reason that is not
  visible in the error.

The failure this avoids is flintlock's existing one: a configuration that names
a client CA, starts cleanly, logs `TLS is enabled`, and verifies nothing.

**16. Every authorization decision is observable.** A structured log line
carrying principal, action, resource, decision and mode, and a Prometheus
counter labelled by decision and mode. `audit` mode exists to let an operator
find out what will break before they flip to `enforce`, and it is useless if
its output is not legible. `grpc_prometheus` is already registered
(`internal/command/run/run.go:215-216`).

**17. A policy may be configured alongside `--insecure`,** with a warning that
states what is actually true: with no client certificates, policy cannot
distinguish callers beyond the token and anonymous kinds. Token authentication
with a namespace-scoped policy is a legitimate posture on a trusted network and
we should not refuse it.

**18. The HTTP/JSON gateway is outside the identity model.** `serveHTTP`
(`internal/command/run/run.go:341-376`) dials flintlock's own gRPC server with
`insecure.NewCredentials()` and forwards no metadata, so every gateway request
resolves to `anonymous` and would be denied under `enforce`. Enabling
`--enable-http` alongside a policy is unsupported. We are not designing a
trusted-proxy identity for a component that is proposed for deprecation in
#1241.

### Compatibility

**19. `--basic-auth-token` is deprecated, not removed.** It keeps working, and
yields the `token` principal — a distinct non-SPIFFE kind, because minting an
SVID-shaped name for an unattested bearer token would misrepresent what was
verified, and because token authentication should not require a trust domain to
be configured.

Removal is gated on the Cluster API provider having migrated (#1240), not on a
version number. Releases here are tag-triggered and ad hoc — six minors between
2026-09-02 and 2026-09-22, after an eighteen-month gap — so a named target
release would slip silently and then either be missed or force a rushed break.

We have no deprecation convention to follow: `Flags().MarkDeprecated` is used
nowhere in the tree, no flag logs a deprecation warning, and the only precedent
is a silent compatibility fallback with an inline comment
(`infrastructure/microvm/cloudhypervisor/state.go:98`). So this ADR also fixes
the mechanics: `MarkDeprecated` on the flag, a warning at startup, and an entry
in the release notes.

## Consequences

**New third-party dependencies.** The repository has no SPIFFE, OPA, Cedar or
Casbin dependency today. `cedar-go` and `go-spiffe` both become direct
dependencies of the root module. This is a genuinely new third-party surface
for a daemon that runs as root.

**`client/` gains `go-spiffe`.** The client is a separate Go module
(`client/go.mod`) that currently depends on only `grpc` and `gomega`, so this
is visible to every downstream consumer including the Cluster API provider. We
keep it single-module, with the constraint that `client/` stays lean. If a real
`go mod` resolution shows a material increase, #1234 splits the auth helpers
into their own module instead. A second module is real ongoing cost — its own
tagging, its own dependency bumps, another `replace` directive — and is not
worth paying on a guess.

**`FLINTLOCKD_TRUST_DOMAIN` does not work until a separate bug is fixed.**
`bindFlagsToViper` (`pkg/flags/flags.go:17-27`) calls `viper.BindEnv` with no
`EnvKeyReplacer`, so viper looks up `FLINTLOCKD_TRUST-DOMAIN`, which is not a
usable POSIX environment variable name. This affects every hyphenated flag
flintlock already has, including `basic-auth-token` and `tls-client-ca`.
Tracked separately.

**`VMID` becomes identity-bearing, and two escape hatches become
security-relevant.** `NewVMIDForce` (`core/models/vmid.go:48`) bypasses all
validation, and `SetUID` (`:110`) mutates a VMID after construction. Both are
fine for a naming key and questionable for an identity. Tracked separately.

**No deployment changes behaviour on upgrade,** because `--authz-mode` defaults
to `off`.

**The first policy an operator writes will lock out legacy callers that omitted
the namespace,** since those land in the default namespace and it is not
special-cased. This needs to be prominent in the configuration documentation
(#1239).

**Caller identity enters flintlock's logs for the first time.** A daemon that
currently logs nothing about who called will log a principal per decision. That
has privacy and log-volume implications worth being deliberate about.

**`SECURITY.md`'s scope statement becomes enforceable.** "Beyond what the API
is meant to allow" only means something once the API means to allow different
things to different callers.

**gRPC reflection is the same category of question as `ServerInfo`** and should
end up answered the same way. `reflection.Register` is unconditional
(`internal/command/run/run.go:240`); gating it stays with #1238.

**Issues created as a result of this ADR:** the viper environment-variable
binding bug; the guest SVID issuance ADR; Cedar policy hot-reload; hardening
`NewVMIDForce` and `SetUID`; correcting the stale ADR-numbering instruction in
`CONTRIBUTING.md`; and correcting ADR 0003's status, which still reads
`Proposed` although it was implemented.
