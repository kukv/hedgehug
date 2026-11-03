---
kind: story
children:
- {title: 設計, assignees: inherit, owner: lead} # 未知のキー
# ブロックの子
- title: 実装
  estimate_hint: 3d # 未知のキー
  labels: [backend]
  children:
  - {title: API, custom: 1}
  - title: 2026-11-03
    props: {points: 2}
- {title: レビュー}
---
本文
