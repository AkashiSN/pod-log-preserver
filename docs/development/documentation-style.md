# Documentation style guide

This guide is the shared writing standard for humans and AI agents that create
or modify project documentation. Its purpose is to keep documentation accurate,
consistent, and easy to use without forcing every document into the same shape.

## 1. Scope and authority

Apply this guide to:

- Markdown under `docs/`
- `README.md` and `README.ja.md`
- documentation sections in contributor-facing files such as `CONTRIBUTING.md`

Agent adapter files such as `.kiro/steering/*.md`, `AGENTS.md`, and `CLAUDE.md`
may point to this guide, but must not duplicate it as a second source of truth.

This guide controls presentation and organization. It does not override:

1. the canonical product behavior in `docs/specification/`
2. the repository instructions and architectural invariants in `AGENTS.md`
3. the contributor process in `CONTRIBUTING.md`
4. document-specific conventions already established in the page being edited

If two sources conflict, preserve the higher-authority source and report the
conflict. Do not silently resolve a design or process conflict as a style edit.

The keywords **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY**
indicate requirement strength. Requirements marked **MUST** are correctness or
project-consistency rules. Guidance marked **SHOULD** may be departed from when
clarity or technical accuracy requires it.

## 2. Core writing principles

- **Accuracy before brevity.** Never omit a condition, limitation, or exception
  merely to shorten a paragraph, list, table, or diagram.
- **Reader task before background.** State what the reader needs to know, decide,
  or do before giving supporting rationale.
- **Structure by meaning.** Use headings, lists, tables, and diagrams when they
  reveal relationships or help navigation, not to meet a numeric layout target.
- **Scannable wording.** Use descriptive headings and lead each section or list
  item with its main point.
- **One source of truth.** Link to canonical behavior or rationale instead of
  copying text that can drift.
- **Minimal necessary change.** Preserve correct terminology, anchors, examples,
  and surrounding structure unless changing them is part of the task.

There are no mandatory limits based on rendered line count, word count, or
diagram source length. Those measures vary by language and editor. Split
content when it contains multiple independent ideas or becomes difficult to
navigate.

## 3. Document profiles

Use the profile that matches the page. If no profile matches, apply the core
principles and follow nearby documents of the same kind.

| Document type | Primary audience | Preferred organization |
|---|---|---|
| Getting Started | First-time users | Outcome, prerequisites, install, verify |
| Specification | Implementers and reviewers | Complete normative behavior |
| Operations / Runbook | Operators | Trigger, signal, action, escalation |
| Development guide | Contributors | Goal, prerequisites, procedure, checks |
| Index page | All readers | Orientation and descriptive navigation |

### 3.1 Getting Started

- Put a minimal successful path and a concrete verification step first.
- Explain concepts after the reader can see the expected outcome.
- Distinguish prerequisites (EKS Auto Mode cluster, fluent-bit deployed) from
  commands the reader must run.
- Include a fluent-bit tail input snippet for the preserved tree so the reader
  has a complete working setup.

### 3.2 Specification

- Precision and completeness take priority over concision.
- State normative behavior separately from rationale and examples.
- Keep terminology, numbering, cross-references, tables, and diagrams aligned
  between the English and Japanese specifications.
- Express edge cases as explicit conditions and outcomes.
- Do not change behavior, configuration keys, metric names, or public surfaces
  without following the Issue-first process in `CONTRIBUTING.md`.

### 3.3 Operations / Runbook

Organize operational procedures around:

- **When:** the symptom, alert, or condition that selects the procedure
- **What to inspect:** the metric, event, log, or command and expected signal
- **What to do:** ordered actions, decision points, and stop conditions
- **When to escalate:** evidence that the procedure is unsafe or insufficient

Keep design rationale in the specification and link to it. Commands MUST make
their assumptions and destructive effects clear.

### 3.4 Development guides

- State prerequisites and the expected result before the procedure.
- Use reproducible commands and identify required tools or environment.
- End with relevant verification and troubleshooting steps.

## 4. Markdown and VitePress conventions

### 4.1 Headings

- Use one `#` page title per page.
- Use `##` for primary page sections, then `###` and `####` without skipping
  levels.
- Make headings descriptive enough to understand in a table of contents.
- Preserve published heading text when changing it would break inbound anchors,
  unless the task includes updating all affected links.

### 4.2 Tables and lists

- Use a table for repeated-field comparison, matrices, and compact lookups.
- Use a list for steps, conditions, exclusions, definitions, or independent
  points.
- Keep a table cell focused on one value or idea. Move multi-paragraph rationale
  below the table.
- Use `- **Label:** explanation` when readers benefit from scanning named items.
- Use numbered lists only when order or sequence matters.

### 4.3 Code and commands

- Add a language identifier to fenced code blocks when one is available.
- Keep commands, identifiers, environment variable names, metric names, YAML
  fields, and program output exact — copy from the implementation source.
- Do not invent command output or claim that a command was run when it was not.
- Explain placeholders and environment-specific values close to the example.
- Environment variable names (e.g. `CLEANUP_INTERVAL_SEC`) MUST appear in
  monospace code formatting in prose and use their exact canonical name.

### 4.4 Links and cross-references

- Prefer descriptive link text over "here" or a bare path in reader-facing prose.
- Within the specification, section references such as `§3.2` MAY supplement a
  descriptive link.
- Use repository-relative links that work on both GitHub and VitePress.
- Link to the canonical source instead of duplicating definitions.
- Every sidebar and nav link in the VitePress config MUST resolve to an existing
  page.

### 4.5 Diagrams

Use a diagram only when it makes a relationship, state transition, decision, or
sequence materially easier to understand:

- `flowchart` for decisions, branching, and system overview
- `sequenceDiagram` for component interactions over time
- `stateDiagram-v2` for states and transitions

Keep each diagram focused on one primary concept. Split it when independent
flows cannot be followed clearly in one view.

VitePress custom containers MAY be used as follows:

- `::: tip` for a concise orientation or key takeaway
- `::: warning` for a limitation, risk, or prerequisite readers must not miss
- `::: details` for supplementary or implementation-level material

Keep the conclusion, primary procedure, and essential safety information visible
without requiring the reader to expand a details block.

## 5. Language and terminology

English is the default language for project documentation. Japanese content
lives under `docs/ja/`, except for `README.ja.md`.

### 5.1 English

- Use active voice when it identifies the actor clearly.
- State facts directly; remove filler such as "It should be noted that".
- Use the project's established terminology consistently:
  - **preserve** (not "backup" or "copy") for the hardlink operation
  - **orphan** for a preserved file with Nlink==1
  - **confirmed** for a file whose tail-DB offset has reached its size
  - **age fallback** (not "timeout" or "expiry") for the mtime-based deletion
  - **tail DB** (not "database", "state file", or "offset DB")
- Keep code identifiers formatted as code.

### 5.2 Japanese

- Use 「ハードリンク」 for hardlink.
- Use 「保全」 for preservation, not 「バックアップ」 or 「退避」 alone
  (though 「退避」 is acceptable as a verb: 「ハードリンクで退避する」).
- Use 「孤児」 for orphan.
- Use 「確認済み」 for confirmed.
- Use 「age しきい値」 or 「age ベースのフォールバック」 consistently — do
  not mix with 「タイムアウト」 or 「有効期限」.
- Use 「デフォルト」 for default — do not mix with 「既定」 or 「既定値」.
- Use 「ローテート」 for rotate/rotated in the kubelet context.
- Keep `pod-log-preserver`, `fluent-bit`, environment variable names, metric
  names, SQL queries, file paths, and CLI commands in English.
- Show durations with concrete syntax such as `60` (seconds) or `5` (minutes)
  matching the env var semantics.
- Use Japanese headings in translated specifications.
- Use descriptive links in user-facing pages; section-only references (§3.2)
  are acceptable inside the specification.

In Japanese fenced code blocks, translate comments only. Commands, identifiers,
YAML keys and values, and program output MUST remain identical to the English
source.

### 5.3 Metric name references

When referencing a Prometheus metric in prose, always use the full metric name
including the `pod_log_preserver_` prefix (e.g.
`pod_log_preserver_fluentbit_db_errors_total`). Do not abbreviate or drop the
prefix, as readers may search for the metric name as written.

## 6. Translation policy

| English source | Japanese counterpart | Same-PR obligation |
|---|---|---|
| `docs/specification/` | `docs/ja/specification/` | **MUST** update |
| `docs/index.md` | `docs/ja/index.md` | **MUST** update |
| Getting Started (when created) | `docs/ja/getting-started.md` | **MUST** update |
| Other docs under `docs/` | Existing page under `docs/ja/`, if any | Update when the task requires it |

For the specification:

- headings, section numbering, tables, and diagram structure MUST remain aligned
  between English and Japanese
- translated labels and prose MAY differ in length
- identifiers, env var names, metric names, and commands MUST remain exact
- both languages MUST describe the same behavior, status, and limitations

### 6.1 Terminology consistency across locales

The following terms MUST be translated consistently:

| English | Japanese | Notes |
|---|---|---|
| preserve / preservation | 保全 | Verb form: 「保全する」 |
| hardlink aside | ハードリンクで退避 | |
| orphan | 孤児 | |
| confirmed (consumed) | 確認済み（収集済み） | |
| age fallback | age フォールバック / age ベースのフォールバック | |
| age threshold | age しきい値 | Not 「閾値」 or 「タイムアウト」 |
| default | デフォルト | Not 「既定」 or 「既定値」 |
| tail DB | tail DB | Keep in English |
| cleanup | クリーンアップ | |
| resync | resync / full resync | Keep in English |
| watch directory | watch ディレクトリ | |
| preserve directory | 保全ディレクトリ | |
| fail fast | 早期に失敗 / fail-fast | Either form acceptable |
| distroless static | distroless static | Keep in English |

## 7. Safe change process

Before editing:

1. identify the page's audience and document profile
2. read the relevant canonical specification and nearby related pages
3. determine whether the change affects behavior or only presentation

While editing:

- preserve technical meaning and distinguish current, planned, and validated
  behavior
- do not make broad rewrites solely to enforce stylistic uniformity
- do not fabricate design decisions, compatibility claims, test results, links,
  commands, or examples
- update affected cross-references and mandatory translations in the same change
- report ambiguity or source conflicts instead of guessing

After editing, review the diff as a reader and verify that:

- the requested information is easy to find
- prerequisites, limitations, and destructive actions remain visible
- terminology and links are consistent
- no unrelated content was rewritten

## 8. Validation and definition of done

Run the checks relevant to the changed files:

```bash
make docs-build    # or: npm run docs:build (from docs/)
git diff --check
```

The documentation change is complete when:

- the rendered documentation builds without new broken links or Markdown errors
- mandatory EN/JA counterparts are synchronized
- commands and examples have been checked against the implementation or an
  authoritative source
- every sidebar/nav link in `docs/.vitepress/config.mts` resolves to an
  existing file
- any check that could not be run is reported with the reason

## 9. Adding support for another AI agent

Keep this file agent-independent. When the project adopts another AI agent, add
a small adapter in that agent's native instruction location rather than copying
this guide.

An adapter SHOULD:

1. activate only for the documentation files covered by this guide when the
   agent supports path-based matching
2. require the agent to read this file in full before editing
3. name the protected areas: authority, translation, safe changes, and validation
4. explain that inconsistent output can break specification accuracy,
   operational safety, or EN/JA synchronization
5. identify this file as the single source of truth

Keep adapter content short. A duplicated guide will drift and turn agent-specific
files into competing sources of truth.
