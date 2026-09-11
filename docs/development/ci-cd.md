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

| Tool | Purpose |
|------|---------|
| `golang/go` | Compile and test |
| `golangci/golangci-lint` | Static analysis |
| `helm/helm` | Chart lint, template, package |
| `kubernetes-sigs/kind` | e2e kind cluster |
| `kubernetes/kubernetes/kubectl` | e2e cluster interaction |

The table deliberately carries no version column. Every one of these pins is
bumped by Renovate (see below), so a copy written here is stale from the next
merge onward — and a page claiming `aqua.yaml` is the single source of truth is
the worst place to keep a second one. Read the versions from `aqua.yaml`.

## Dependency updates: Renovate groups

Renovate (`.github/renovate.json`) opens the dependency PRs. Its `packageRules`
exist for one reason: some of the pins above are **coupled**, and a PR that
moves one without the others cannot pass CI. Each group is one PR.

| Group | Members | Why they travel together |
|---|---|---|
| `go stack` | `gomod` (module requires + the `go` directive), `dockerfile` (golang builder + distroless runtime), `golang/go`, `golangci/golangci-lint` | The Go version is pinned in three places (`go.mod`, `Dockerfile`, `aqua.yaml`) and `check-go-toolchain-sync.sh` fails the build if they disagree. golangci-lint refuses to run when its own build Go minor is older than the module's. |
| `e2e cluster` | `kubernetes-sigs/kind`, `kubernetes/kubernetes/kubectl` | Bumping kind changes the cluster's Kubernetes version, and kubectl tracks that node's Kubernetes minor. |
| `dev tooling` | `aquaproj/aqua`, `aquaproj/aqua-registry`, `aquaproj/aqua-renovate-config`, `helm/helm` | Not coupled to anything — batched only to cut PR volume. |
| `docs site` | `npm` (vitepress, mermaid, vue, …) | All validated by the same `npm run docs:build`. `mermaid` is additionally capped at `<12` — see below. |
| `github actions` | `github-actions` | All validated by CI running at all. |

::: warning The `go` directive needs `rangeStrategy: bump`
The `go` directive in `go.mod` is a *minimum*, not a pin. Under Renovate's default
`replace` strategy every newer Go still satisfies it, so Renovate proposes no
update and silently leaves the directive behind while it moves `aqua.yaml` and
the `Dockerfile` — the exact drift `check-go-toolchain-sync.sh` rejects. The
`rangeStrategy: "bump"` rule on the `gomod` / `golang` dep type is what makes
the directive move with the other two pins.
:::

::: tip Why `kind.yaml` pins no node image
kind's default node image is compiled into the binary and pinned by digest, so
it already moves in lockstep with the `kubernetes-sigs/kind` version in
`aqua.yaml`. A tag written into `test/e2e/kind/kind.yaml` would instead have to
be re-resolved by hand on every bump — no Renovate manager can derive it from
the kind version — which is exactly the drift that reached `main` in #93.
Omitting the override removes the coupling rather than documenting it.

To see which node image the pinned kind will use:

```console
$ strings "$(aqua which kind)" | grep -o 'kindest/node:v[0-9.]*'
```
:::

::: warning `mermaid` is capped at `<12`
`vitepress-plugin-mermaid` declares `peerDependencies: { mermaid: "10 || 11" }`
and 2.0.17 is its latest release, so v12 has no compatible plugin yet.

The failure mode is asymmetric and easy to misread: `npm install` only *warns*
about the conflict and overrides it, so a v12 lockfile can be written and
`npm run docs:build` even succeeds — but `npm ci`, which is what
`docs-lint.yaml` runs, enforces the peer range and fails the resolution
outright. A v12 bump therefore looks locally fine and is unmergeable in CI.

Raise or drop the `allowedVersions` rule in `.github/renovate.json` as soon as
the plugin admits v12.
:::
