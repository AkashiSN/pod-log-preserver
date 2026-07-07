# pod-log-preserver

EKS Auto Mode 上で kubelet がローテートした Pod ログを、ログエージェントが収集し終えるまで保全し、その後ディスクを自動で回収するツール。

## なぜ必要か

EKS Auto Mode では kubelet の `containerLogMaxSize`（10MB）と `containerLogMaxFiles`（5）をカスタマイズできない。ログの書き込みがログエージェントの収集より速いコンテナでは、ローテートされたログが読み取られる前に kubelet に削除され、その行が失われることがある。`pod-log-preserver` はこの隙間を埋める。

## 仕組み

DaemonSet として動作し、`/var/log/pods` を監視して各 Pod ログを同一ファイルシステム上の保全ディレクトリに**ハードリンク**する。これにより kubelet が元ファイルを削除してもバイト列は生き続ける。クリーンアップループはログエージェント（fluent-bit）の tail DB を**読み取り専用**で参照し、エージェントが読み終えた保全ファイルだけを削除する。未確認のファイルは age 閾値によるフォールバックで削除される。詳細な設計は[仕様書](docs/ja/specification/)を参照。

## インストール（Helm）

> **予定 — 最初のリリース `v0.5.0` から提供。** イメージ（`ghcr.io/akashisn/pod-log-preserver`）と OCI Helm chart（`oci://ghcr.io/akashisn/charts/pod-log-preserver`）はリリースワークフローで publish され、実装とともに追加される。それまで本リポジトリは仕様と開発プロセスのみを提供する。

```bash
helm install pod-log-preserver \
  oci://ghcr.io/akashisn/charts/pod-log-preserver --version 0.5.0 \
  --namespace kube-system
```

## 設定

すべて環境変数で設定する（[仕様 §5.4](docs/ja/specification/05-implementation.md#54-設定スキーマ)を参照）。

| 環境変数 | 既定値 | 意味 |
|---------|-------|------|
| `WATCH_DIR` | `/var/log/pods` | 監視するディレクトリツリー |
| `PRESERVE_DIR` | `/var/log/pods-preserved` | ハードリンクの作成先 |
| `CLEANUP_INTERVAL_SEC` | `60` | クリーンアップループの周期 |
| `CLEANUP_MAX_AGE_MIN` | `5` | `.gz` 以外の orphan の age 閾値 |
| `CLEANUP_GZ_MAX_AGE_MIN` | `60` | `.gz` の orphan の age 閾値 |
| `RESYNC_INTERVAL_SEC` | `30` | 定期フルリシンクの周期 |
| `NAMESPACE_FILTER` | （空 = すべて） | カンマ区切りの namespace glob パターン |
| `LOG_LEVEL` | `info` | `debug` または `info` |
| `METRICS_PORT` | `9113` | Prometheus メトリクスのポート |
| `PRESERVED_LOG_DB_GLOB` | `/var/lib/fluent-bit/flb_kube*.db` | tail DB の glob。空で DB 連携クリーンアップを無効化 |

## メトリクス

`METRICS_PORT` の `/metrics` で Prometheus エンドポイントを提供し、`pod_log_preserver_preserved_files`・`..._orphaned_files`・`..._preserved_bytes`・`..._hardlinks_created_total`・`..._orphans_removed_total`・`..._db_confirmed_removed_total`・`..._fluentbit_db_errors_total` を公開する。[仕様 §4.2](docs/ja/specification/04-operations.md#42-可観測性)を参照。

## 要件・注意点

- **同一ファイルシステム**: 監視ディレクトリと保全ディレクトリは同一ファイルシステム上になければならない（ハードリンクはファイルシステムを越えられない）。起動時のテストで担保する。
- **root 必須**: kubelet 所有のログの読み取りとハードリンク作成に uid 0 が必要。distroless の `nonroot` タグは使えない。
- **tail DB は読み取り専用だが rw マウント**: fluent-bit は WAL を使い、WAL reader は `-shm` インデックスへの登録に DB ディレクトリへの書き込み権限を要する。

詳細は[仕様 §4](docs/ja/specification/04-operations.md)を参照。

## ライセンス

[Apache License 2.0](LICENSE)。
