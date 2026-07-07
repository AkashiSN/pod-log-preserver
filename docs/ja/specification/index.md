# pod-log-preserver — 仕様書

EKS Auto Mode 上で kubelet がローテートした Pod ログを退避・保全する Kubernetes DaemonSet の機能仕様。ログエージェント（fluent-bit）が収集し終えるまでハードリンクで保全し、その後クリーンアップする。

English: [docs/specification/](../../specification/)

---

## 目次

1. **[概要](./01-overview)** — 背景・ゴール・非ゴール・用語
2. **[スコープ](./02-scope)** — 対応環境・fluent-bit との組み合わせ
3. **[設計](./03-design)** — 保全・tail DB 確認クリーンアップ・age フォールバック・namespace フィルタ
4. **[運用](./04-operations)** — 要件・可観測性・RBAC/セキュリティ・コスト
5. **[実装](./05-implementation)** — アーキテクチャ・イベントループ・クリーンアップループ・設定スキーマ
6. **[リリース](./06-release)** — バージョニング・ロードマップ
7. **[リスクと状況](./07-risks)** — リスク・検証済みの前提・未解決の論点

## 参考資料

- [EKS Auto Mode ドキュメント](https://docs.aws.amazon.com/eks/latest/userguide/automode.html)
- [Kubernetes ノードのログ（containerLogMaxSize / containerLogMaxFiles）](https://kubernetes.io/docs/concepts/cluster-administration/logging/#log-rotation)
- [fluent-bit tail input](https://docs.fluentbit.io/manual/pipeline/inputs/tail)
