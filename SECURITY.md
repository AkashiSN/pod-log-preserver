# Security Policy

Thanks for helping keep `pod-log-preserver` and its users safe.

## Supported versions

The project is **pre-1.0** (`v0.x.y`); see the
[release roadmap](docs/specification/06-release.md#62-roadmap) for the path to a
stable v1.0. Security fixes are applied to the **latest released minor only** —
there are no long-term-support branches before 1.0. Always run the most recent
release.

| Version | Supported |
|---------|-----------|
| Latest `0.x` release | ✅ |
| Any older release | ❌ — upgrade to the latest |

## Reporting a vulnerability

**Please report security issues privately. Do _not_ open a public GitHub issue,
pull request, or Discussion for a suspected vulnerability** — that discloses it
before a fix is available.

Report it through GitHub's **private vulnerability reporting**:

1. Go to the repository's **Security** tab.
2. Click **Report a vulnerability** (or open
   <https://github.com/AkashiSN/pod-log-preserver/security/advisories/new>).
3. Describe the issue with enough detail to reproduce it (see below).

This opens a private advisory visible only to you and the maintainers.

> If private vulnerability reporting is not yet enabled on the repository, a
> maintainer can turn it on under **Settings → Code security and analysis →
> Private vulnerability reporting**. Until then, please wait rather than
> disclosing publicly.

### What to include

- A clear description of the issue and its security impact.
- The affected version(s) — chart version and/or image tag.
- Steps to reproduce, ideally a minimal DaemonSet / values setup.
- Any relevant logs, manifests, or configuration (redact secrets and
  account-identifying details).

### What to expect

This is a small, volunteer-maintained open-source project, so responses are
best-effort rather than bound by an SLA. We aim to:

- **acknowledge** your report within about a week;
- **assess and trial a fix** privately, keeping you updated on progress;
- **release a fix** and publish a [GitHub Security Advisory](https://github.com/AkashiSN/pod-log-preserver/security/advisories)
  crediting you (unless you prefer to stay anonymous), then disclose publicly.

We follow **coordinated disclosure**: please give us a reasonable window to ship
a fix before any public write-up.

## Scope

In scope — vulnerabilities in this project's own code and packaging:

- the log-preserver program and its container image;
- the Helm chart, including its hostPath mounts and DaemonSet security context;
- privilege escalation, unsafe file deletion under the preserve directory, or
  reads outside the intended log paths introduced by the above.

Out of scope — report these to their respective projects/vendors:

- **fluent-bit** itself and its tail-DB format;
- **Kubernetes**, EKS Auto Mode, or any cloud-provider control plane;
- a cluster operator's own misconfiguration (e.g. mounting a preserve directory
  on a different filesystem than the log source, which the startup hardlink test
  rejects) that the program faithfully reports.

Operational guidance for safe configuration lives in the
[specification](docs/specification/04-operations.md).
