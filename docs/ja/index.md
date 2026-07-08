---
layout: home
hero:
  name: pod-log-preserver
  text: EKS Auto Mode で kubelet がローテートした Pod ログを保全
  tagline: ローテートされた Pod ログを、ログエージェントが収集し終えるまでハードリンクで退避し、その後ディスクを自動で回収する。
  actions:
    - theme: brand
      text: はじめに
      link: /ja/getting-started
    - theme: alt
      text: 仕様書
      link: /ja/specification/
features:
  - title: 収集確認後のクリーンアップ
    details: fluent-bit の tail DB を読み取り専用で参照し、エージェントが読み終えた保全ログのみを削除する。未確認のものは age ベースのフォールバックで削除する。
  - title: データコピーはゼロ
    details: 同一ファイルシステム上でハードリンクにより保全するため、追加のデータブロックを消費しない。収集が確認された瞬間にディスクを回収する。
  - title: 自己完結型 DaemonSet
    details: distroless static イメージ上の単一 pure-Go バイナリ。依存は SQLite のみで、Kubernetes API へのアクセスは不要。
---
