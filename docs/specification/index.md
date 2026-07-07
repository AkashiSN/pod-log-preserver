# pod-log-preserver — Specification

Functional specification for a Kubernetes DaemonSet that preserves
kubelet-rotated pod logs on EKS Auto Mode — hardlinking them aside until a log
agent (fluent-bit) has collected them, then cleaning them up.

Japanese translation: [docs/ja/specification/](../ja/specification/)

---

## Contents

1. **[Overview](./01-overview)** — Background, Goals, Non-Goals, Terminology
2. **[Scope](./02-scope)** — Supported environments, Composition with fluent-bit
3. **[Design](./03-design)** — Preservation, Tail-DB-confirmed cleanup, Age fallback, Namespace filter
4. **[Operations](./04-operations)** — Requirements, Observability, RBAC/security, Cost
5. **[Implementation](./05-implementation)** — Architecture, Event loop, Cleanup loop, Configuration schema
6. **[Release](./06-release)** — Versioning, Roadmap
7. **[Risks & Status](./07-risks)** — Risks, Validated assumptions, Open questions

## References

- [EKS Auto Mode docs](https://docs.aws.amazon.com/eks/latest/userguide/automode.html)
- [Kubernetes node logging (containerLogMaxSize / containerLogMaxFiles)](https://kubernetes.io/docs/concepts/cluster-administration/logging/#log-rotation)
- [fluent-bit tail input](https://docs.fluentbit.io/manual/pipeline/inputs/tail)
