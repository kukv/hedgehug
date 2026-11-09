# 同期

`hh sync` は、fetch、マージ、修復、push をまとめて行います。ふだんは実行しなくても構いません。remote のある Store は裏で同期するからです。書き込みの約30秒後に push し（`sync.push_interval`）、前回の同期から2分以上たっていれば、どのコマンドでも同期を始めます（`sync.interval`）。どちらも `config.yaml` で変えられます。すぐに同期したいときや、結果を見たいときに `hh sync` を実行してください。`HEDGEHUG_AUTO_SYNC=0` で裏の同期を止められます。

```console
$ hh sync --status
team
  last attempt:  2026-11-06T11:28:54+09:00
  last success:  2026-11-06T11:28:54+09:00
  pending since: 2026-11-06T11:29:12+09:00
  2 commits not pushed
$ hh sync
Synced team: 1 merge, pushed to origin
```

`hh sync` は、remote を持つすべての Store を同期します。`--store <name>` で1つに絞れます。`--status` は直近の同期の結果を表示するだけで、同期はしません。GUI を開いている間は、GUI が自分で同期します。

## マージの仕組み

同じ Store を2人がオフラインで編集しても、次の同期で、hedgehug がファイルをフィールドごとにマージします。同じタスクの別々のフィールドを編集しても衝突せず、コメントは1件ごとに新しいファイルになります。規則が要るのは次の2つです。

- **同じフィールドが両側で変わったとき。** あとの変更が残ります。前の値は失われず、マージコミットに記録されます。
- **同じ ID が2回振られたとき。** あなたと同僚がオフラインで、どちらも `WEB-3` を作ったとします。片方がその ID を持ち、もう片方は振り直されます。タスクの恒久的な名前である ULID は変わりません。

同期はこの2つを表示し、`hh inbox` は読むまで残します。

```console
$ hh sync
Synced team: 1 merge, pushed to origin
  WEB-3 (01M6QB0KC5488AWY7AS44Y67JY) → WEB-4
  overwritten: WEB-1 status (github.com/alice's value was replaced)
```

## 自分の値が上書きされたとき

負けた側の人は、次の `hh inbox` で知ります。

```console
$ hh inbox
01M6QB0TGFH3B29VE6C5CN04ME  overwritten  WEB-1 (...) Write the login page  status: "waiting" was replaced
```

自分の値を戻すには、1列目の ID を `hh inbox revert` に渡します。

```console
$ hh inbox revert 01M6QB0TGFH3B29VE6C5CN04ME
Reverted WEB-1 (01M6QAZMHYRXKQJSTS7KTHA2GC) Write the login page
```

`hh inbox read <ULID>...` は項目を既読にします。`hh inbox --all` は既読の項目も表示します。

## 直前の操作を取り消す

```console
$ hh undo
Undid fd95324 status WEB-2 Fix the logout bug
```

`hh undo` は自分の直前の操作を取り消し、そのあとにほかの人が加えた変更は残します。新しいコミットを作るので、履歴は書き換えません。

## 1つのタスクを以前の状態に戻す

`hh restore <id> --at <時刻かコミット>` は、1つのタスクを指定した時点の状態に戻します。時刻は RFC 3339 の形式か日付で、日付はその日の終わりを指します。コミットは `hh log <id>` で調べます。

```console
$ hh log WEB-1
a5bba5a  2026-11-06 11:29  alice  hedgehug: set WEB-1 (...) Write the login page
31f15d5  2026-11-06 11:28  alice  hedgehug: set WEB-1 (...) Write the login page
09431e7  2026-11-06 11:28  alice  hedgehug: add WEB-1 (...) Write the login page
$ hh restore WEB-1 --at 31f15d5
Restored WEB-1 (01M6QAZMHYRXKQJSTS7KTHA2GC) Write the login page
```

タスクの ID とワークスペースは、今のままです。`--at` なしの `hh restore <id>` は、`hh rm` で消したタスクを、同じ `rm` で消えた子孫ごと戻します。

## remote が新しい形式になったとき

ほかのクローンが `hh store convert` を実行して push したあと、push していないコミットが残っているクローンは、それを取り込めません。そのクローンは古い形式なので、`hh sync` は `store.old_format`（HTTP 422）で止まり、`hh store convert` を実行するよう案内します。実行してください。push していない変更を新しい形式に変換し、remote と合わせて push します。push していないコミットのないクローンも同じ操作で、`hh store convert` が fast-forward するだけです。[Store](stores.md#store-の形式と-hh-store-convert)を参照してください。

逆向きに、変換したが push していないクローンが、別のクローンが古い形式のコミットを push したことに気づくこともあります。そのとき `hh sync` は `sync.format_diverged`（HTTP 409）で止まり、メッセージは remote に古い形式の変更があることを知らせます。`hh store convert` を実行してください。remote の変更を変換し、remote の最新のコミットの上のコミットとして push して、手元の変更とマージします。変換の前に、その Store を使うすべてのクローンで hedgehug を更新してください。[Store](stores.md#変換の前に)を参照してください。

## origin の履歴が変わったとき

remote の履歴が書き換えられると、`hh sync` は push せずに止まります。対処は[メンテナンス](maintenance.md#履歴から内容を消す)にあります。
