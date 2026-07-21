# CI/CD Design

::: tip What this page covers
How this repository keeps required status checks fast without ever leaving one stuck `pending` — via step-level gating driven by a centralized change-detection classifier.
:::

## Workflows

| Workflow | Trigger | Purpose |
|---|---|---|
| `ci.yaml` | push to `main`, every PR | Required: `changes`, `lint`, `test`, `docker`, `chart` |
| `e2e.yaml` | push to `main`, every PR | Container harness + kind smoke test |
| `release.yaml` | push of a `v*` tag | Multi-arch image + Helm chart OCI + GitHub Release |
| `pages.yaml` | push to `main` (docs paths), manual | VitePress site → GitHub Pages |
| `docs-lint.yaml` | push to `main`, every PR | Markdown lint and link checks |

## The `pending` trap

::: warning Why this matters
A required check with no conclusion is not "green" — it is stuck `pending`. A PR can never merge while any required check sits in that state.
:::

Branch protection on `main` names exact required status checks. If a workflow or
job is **skipped outright** (via `paths-ignore` or job-level `if:`), GitHub never
reports a conclusion → stuck `pending`.

**Solution:** always run the *job*, skip only its expensive *steps* when nothing
they care about changed.

- Every step that matters carries a per-step `if:`
- The job always reaches a real conclusion (success) in seconds when inputs are untouched
- Full work runs only when relevant files change

This is why the repository does **not** use `paths-ignore` or job-level `if:`.

## `ci.yaml`: the `changes` job

### How it works

1. A dedicated `changes` job computes gating flags (always runs, no `if:`)
2. `lint`, `test`, `docker`, `chart` each declare `needs: changes` and read its outputs
3. Classification logic lives in one place (DRY)

The classifier is `.github/scripts/detect-ci-changes.sh`:
- A small, pure shell script (no `git`, no GitHub Actions context)
- Reads newline-separated changed paths on stdin
- Prints four booleans: `go`, `chart`, `docker`, `infra`

### Input sources

| Context | Input |
|---------|-------|
| Pull request | `git diff --name-only "$BASE_SHA" HEAD` |
| Push to `main` (or `workflow_dispatch`) | All flags `true` (no base to diff; always run everything) |

### Self-test

`.github/scripts/detect-ci-changes.test.sh` unit-tests the classifier against a
table of sample path sets. It runs on every CI invocation — gating logic cannot
silently rot.

The same `changes` job also runs `.github/scripts/check-go-toolchain-sync.sh`,
which asserts the Go version pinned in `aqua.yaml` matches `go.mod`. A PR
cannot introduce a mismatch.

### Path → flag → job

| Path pattern | Flag | Gated jobs/steps |
|---|---|---|
| `*.go`, `go.mod`, `go.sum`, `internal/`, `cmd/` | `go` | `lint`, `test`, `docker` |
| `charts/**` | `chart` | `chart` |
| `Dockerfile`, `.dockerignore` | `docker` | `docker` |
| `Makefile`, `aqua.yaml`, `.github/workflows/ci.yaml`, `.github/scripts/**`, `.golangci.yml` | `infra` | `lint`, `test`, `docker`, `chart` |

### Resulting step gates

| Job | Runs real steps when |
|-----|---------------------|
| `lint` | `go \|\| infra` |
| `test` | `go \|\| infra` |
| `docker` | `go \|\| docker \|\| infra` |
| `chart` | `chart \|\| infra` |

`infra` is deliberately broad: CI workflow, Makefile, or aqua toolchain pins can
affect all jobs, so it fans out to them rather than guessing.

## `ci.yaml` job details

### lint

Runs `make lint` (golangci-lint, pinned in aqua.yaml).

### test

Runs `make build` then `make test` (Go test suite with race detector).

### docker

Multi-arch build (`linux/amd64`, `linux/arm64`) with `push: false` — proves the
pure-Go cross-compile and the distroless-static runtime assemble without
publishing.

### chart

1. `helm lint` — catches structural issues
2. `helm template` — catches template/schema errors that lint misses
3. `helm unittest` — rendered-manifest assertions proving the load-bearing
   invariants (hostPath mounts, root securityContext, env wiring) match the spec
4. Schema rejection tests — `values.schema.json` negative cases assert that
   invalid values (typos, out-of-range) are rejected at render time

## `release.yaml`: tag-driven OCI publish

Triggers only on `v*` tags — not part of branch protection, so no
`pending`-check exposure.

### Four sequential jobs

| Job | Purpose |
|-----|---------|
| `guard` | Fail if tag disagrees with `Chart.yaml` version/appVersion |
| `image` | Multi-arch build + push + verify embedded VERSION matches tag |
| `chart` | Package Helm chart → OCI push to GHCR |
| `release` | Create GitHub Release with the chart `.tgz` attached |

### Guard checks

- `check-chart-version.sh` requires tag == chart `version` == `appVersion`
- The image job verifies `internal/version/VERSION` matches the tag

### Image details

- Architectures: `linux/amd64`, `linux/arm64`
- Registry: `ghcr.io/akashisn/pod-log-preserver`
- Tags: the `v*` tag, plus `latest` (skipped for hyphenated pre-release tags)
- No provenance/SBOM attestation index (plain multi-arch manifest)

### Chart details

- Packages from committed `Chart.yaml` (no `--version` override)
- Pushes to `oci://ghcr.io/akashisn/charts`
- The `.tgz` is uploaded as a build artifact for the release job

### Release details

- Downloads the packaged chart artifact
- Creates a GitHub Release with auto-generated release notes
- Marks as pre-release for hyphenated tags (e.g. `v0.6.0-rc.1`)

## `pages.yaml`: docs deploy

Builds the VitePress site with `npm run docs:build` and deploys to GitHub Pages.

Not a required check — publishes docs, does not gate merges.

## Tooling: aqua as single source of truth

All CLI versions (Go, golangci-lint, helm, kind, kubectl) are pinned in
`aqua.yaml`. There is no `actions/setup-go`, `golangci-lint-action`, or
`azure/setup-helm` — aqua-installer installs aqua and links the commands; `make`
lazily installs each pinned version on first use. Local `make` and CI use
byte-identical tools.

| Tool | Pinned version | Purpose |
|------|---------------|---------|
| `golang/go` | `go1.26.5` | Compile and test |
| `golangci/golangci-lint` | `v2.12.2` | Static analysis |
| `helm/helm` | `v4.2.3` | Chart lint, template, package |
| `kubernetes-sigs/kind` | `v0.32.0` | e2e kind cluster |
| `kubernetes/kubectl` | `v1.36.2` | e2e cluster interaction |
