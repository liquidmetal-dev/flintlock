# Security Policy

## Supported versions

Only the [latest minor release](https://github.com/liquidmetal-dev/flintlock/releases/latest)
of flintlock receives security fixes. If you are running an older release,
please upgrade to the latest release to pick up fixes.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues,
pull requests or discussions.**

Instead, report them privately using GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/liquidmetal-dev/flintlock/security)
   of this repository.
2. Click **Report a vulnerability**, or go directly to
   <https://github.com/liquidmetal-dev/flintlock/security/advisories/new>.

To help us triage your report quickly, please include as much of the
following as you can:

- The flintlock version (or commit) affected
- The VMM in use (Firecracker or Cloud Hypervisor) and its version
- Relevant `flintlockd` configuration (e.g. TLS and authentication settings)
- Steps to reproduce, ideally with a proof of concept
- The impact, and what an attacker would need in order to exploit it

## What to expect

- We will acknowledge your report within **5 business days**.
- We will investigate, confirm whether the issue is a vulnerability, assess
  its severity and keep you updated on progress.
- Fixes are developed privately in a temporary private fork linked to the
  GitHub Security Advisory, and you are welcome to collaborate on the fix there.

## Disclosure and CVEs

We follow coordinated disclosure:

- We aim to release a fix typically within 90 days of the report.
- The security advisory is published when the release containing the fix is
  available.
- We ask that you do not disclose the vulnerability publicly until the
  advisory is published, or for up to 90 days from your report, whichever
  comes first. This window can be extended by mutual agreement if a fix
  needs more time.
- Severity is assessed using [CVSS](https://www.first.org/cvss/), and CVE IDs
  are requested through GitHub as the CVE Numbering Authority.

## Scope

In scope:

- `flintlockd`, its gRPC API and the API's authentication
- The Go client and `pkg` libraries in this repository
- Release artifacts and the install/provisioning scripts in this repository

In scope specifically: bypassing basic-auth or TLS client validation when they
*are* enabled, and any issue reachable by an authenticated client that goes
beyond what the API is meant to allow (e.g. escaping the image mount via
kernel/initrd paths).

Out of scope:

- Vulnerabilities in Firecracker, Cloud Hypervisor, containerd or the Linux
  kernel themselves. Please report these to the relevant upstream project.
- Issues that require an already-compromised host or existing root access.
- Attacks that require running `flintlockd` with `--insecure` and no
  `--basic-auth-token` on a network the attacker can reach. Running without
  TLS and authentication is not recommended outside of local development.

## Credit

We are happy to credit reporters in the published advisory; let us know in
your report if you would like to be credited, and how.

flintlock does not offer a bug bounty.
