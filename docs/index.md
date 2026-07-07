---
layout: home
hero:
  name: pod-log-preserver
  text: Preserve kubelet-rotated pod logs on EKS Auto Mode
  tagline: Hardlink rotated pod logs aside until a log agent has collected them, then reclaim disk automatically.
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started
    - theme: alt
      text: Specification
      link: /specification/
features:
  - title: Collection-confirmed cleanup
    details: Reads fluent-bit's tail DB read-only and deletes a preserved log only once the agent has read it fully — with an age-based fallback when unconfirmed.
  - title: Zero data copies
    details: Preserves logs via hardlinks on the same filesystem, so it adds no extra data blocks and reclaims disk the moment collection is confirmed.
  - title: Self-contained DaemonSet
    details: A single pure-Go binary on a distroless static image; SQLite is the only dependency and no Kubernetes API access is required.
---
