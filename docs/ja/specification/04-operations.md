# 4. 運用

## 4.1 要件・注意事項

| 項目 | 対応 |
|---------|-----------|
| 同一ファイルシステム | watch ディレクトリと保全ディレクトリは同一ファイルシステムを共有しなければならない。そうでない場合、起動時のハードリンクテストが早期に失敗する。 |
| root 権限が必要 | kubelet 所有のログの読み取りとハードリンクの作成には uid 0 が必要である。distroless の `nonroot` タグは使用できない。 |
| hostPath マウント | `/var/log`（rw）とエージェントの DB ディレクトリ（例: `/var/lib/fluent-bit`）はホストからマウントされる。 |
| Tail DB は rw でマウントされる | DB は `mode=ro` で開かれるものの、fluent-bit は WAL を使用しており、WAL リーダーは `-shm` の wal-index に登録する必要があるため、DB ディレクトリへの書き込みアクセスが必要になる。 |

## 4.2 可観測性

Prometheus エンドポイントは `METRICS_PORT`（デフォルト 9113）の `/metrics` で提供される。

| メトリクス | 種類 | 意味 |
|--------|------|---------|
| `pod_log_preserver_preserved_files` | gauge | 現在保全ディレクトリにあるファイル数 |
| `pod_log_preserver_orphaned_files` | gauge | リンクカウントが 1 の保全ファイル数 |
| `pod_log_preserver_preserved_bytes` | gauge | 保全ディレクトリ配下の総バイト数 |
| `pod_log_preserver_hardlinks_created_total` | counter | 作成されたハードリンク数 |
| `pod_log_preserver_orphans_removed_total` | counter | 削除された孤児ファイル数 |
| `pod_log_preserver_db_confirmed_removed_total` | counter | tail DB がフルリードを確認した後に削除された孤児数 |
| `pod_log_preserver_fluentbit_db_errors_total` | counter | Tail DB の読み取りエラー数 |

リスナーは起動時に同期的にバインドされる。`METRICS_PORT` をバインドできない
場合（例: すでに使用中）、エンドポイント無しで動作を続けるのではなく、起動を
fail-fast させる。

ログの詳細度は `LOG_LEVEL`（`debug` / `info`）によって制御される。

## 4.3 RBAC とセキュリティ

- Kubernetes API へのアクセスは不要である。プログラムはノードのファイルシステム上でのみ動作するため、**ServiceAccount の権限 / RBAC は一切不要** である。
- ノードのログディレクトリとエージェントの DB ディレクトリに対する **root** および **hostPath** アクセスが必要である。これは kubelet のログをハードリンクするための最小限の要件である。
- エージェントの tail DB は常に **読み取りのみ**（`mode=ro`、単一コネクション）で扱われる。

## 4.4 コスト

定常状態のフットプリントは小さい（request=limit で CPU 約 50m / メモリ約 32Mi がデフォルトとして妥当である）。保全はハードリンクによって行われるため、追加のデータブロックは消費されず、inode / ディレクトリエントリのオーバーヘッドのみが発生する。ディスクは収集が確認された時点で速やかに解放される。
