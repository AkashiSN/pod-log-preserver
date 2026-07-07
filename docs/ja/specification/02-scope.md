# 2. スコープ

## 2.1 対応環境

### 対応する環境

- **EKS Auto Mode**、および kubelet が `/var/log/pods/<ns>_<pod>_<uid>/<container>/<n>.log` 配下に Pod ログを書き込み、`<n>.log.<timestamp>` / `<n>.log.<timestamp>.gz` にローテートする任意の Kubernetes ノード。
- Linux ノードのみ（イベントループは `inotify` を使用し、保全にはハードリンクを使用する）。
- `x86_64` と `arm64`（イメージはマルチアーキ対応）。

### 要件

- 保全ディレクトリと watch ディレクトリは **同一ファイルシステム** 上に存在しなければならない（ハードリンクはファイルシステムを跨げない）。起動時のハードリンクテストによってこれが強制される。
- プロセスは **root（uid 0）** で実行される。`/var/log/pods` 配下の kubelet 所有ファイルを読み取り、ハードリンクを作成するにはこれが必要である。したがって distroless の `nonroot` イメージタグは使用できない。

## 2.2 ログエージェント（fluent-bit）との組み合わせ

`pod-log-preserver` は fluent-bit の `tail` input と組み合わせて動作するよう設計されている。

- fluent-bit はライブの Pod ログと保全ツリーの両方を tail する。
- `pod-log-preserver` は fluent-bit の tail DB を **読み取り専用** で読み、fluent-bit が各保全ファイル（inode で照合）をどこまで読み進めたかを把握し、fluent-bit がそのファイルを最後まで読み終えた時点で保全ファイルを削除する。
- 保全ファイルがまだいずれの tail DB にも認識されていない場合は、age ベースのクリーンアップに委ねられる。「行が無い」ことを「完了」と解釈して削除することは決してない。

DB のグロブはデフォルトで fluent-bit の命名規則（`flb_kube*.db`）に一致するため、他の tail input に属する DB を誤って読むことはない。この機能はオプションであり、グロブを空にすると、クリーンアップは純粋に age ベースになる。
