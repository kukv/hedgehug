import { ApiError } from "./api/client";

type Args = Record<string, unknown>;
type Make = (a: Args, e: ApiError) => string;

const str = (a: Args, k: string): string => {
  const v = a[k];
  return v === undefined || v === null ? "" : String(v);
};
const list = (a: Args, k: string): string => {
  const v = a[k];
  return Array.isArray(v) ? v.map(String).join("、") : "";
};
// withDetail adds the English message of the server, for errors whose cause
// only the message tells.
const withDetail = (text: string, e: ApiError): string =>
  e.detail ? `${text}: ${e.detail}` : text;

const violationLabels: Record<string, string> = {
  unknown_kind: "未定義の種類です",
  unknown_status: "未定義のステータスです",
  missing_title: "タイトルがありません",
  bad_value: "値が正しくありません",
  placement: "この親には置けません",
  transition: "このステータスには移れません",
  required: "必須の項目が空です",
  missing_ref: "参照先がありません",
  cycle: "循環しています",
};

function violations(a: Args): string {
  const vs = a.violations;
  if (!Array.isArray(vs) || vs.length === 0) return "";
  return vs
    .map((v) => {
      const x = (v ?? {}) as Record<string, unknown>;
      const label = violationLabels[String(x.code)] ?? String(x.code ?? "");
      const field = x.field ? `（${String(x.field)}）` : "";
      return `${label}${field}: ${String(x.message ?? "")}`;
    })
    .join(" / ");
}

// messages has the words for every Code of the ops errors and of the server.
const messages: Record<string, Make> = {
  // tasks and workspaces
  "task.not_found": (a) => `${str(a, "input")} というタスクは見つかりません`,
  "task.ambiguous": (a) =>
    `${str(a, "input")} に当てはまるタスクが複数あります。ULID で指定してください` +
    (list(a, "candidates") ? `（候補: ${list(a, "candidates")}）` : ""),
  not_found: (_, e) => withDetail("見つかりません", e),
  "task.gone": (a) => `${str(a, "id")} はもうストアにありません`,
  "task.deleted": (a) => `${str(a, "id")} は削除済みです。先に復元してください`,
  "task.parent_deleted": (a) => `親の ${str(a, "id")} が削除済みです。先に復元してください`,
  "task.already_deleted": (a) => `${str(a, "id")} はすでに削除されています`,
  "task.not_deleted": (a) => `${str(a, "id")} は削除されていません`,
  "task.workspace_mismatch": () => "親のワークスペースがこの Store にないか、指定と合いません",
  "task.moved": (a) => {
    const input = str(a, "input");
    const store = str(a, "store");
    if (store && a.id) return `${input} は ${store} へ移りました（今の ID は ${str(a, "id")}）`;
    if (store) return `${input} は ${store} へ移りました`;
    return `${input} は別の Store へ移りました`;
  },
  "workspace.not_found": (a) => `${str(a, "input")} というワークスペースは見つかりません`,
  "workspace.ambiguous": (a) => `${str(a, "input")} に当てはまるワークスペースが複数あります`,
  "workspace.taken": (_, e) => withDetail("そのワークスペースの名前はすでに使われています", e),
  "workspace.none": () =>
    "この Store にはワークスペースがありません。hh workspace add で作ってください",
  "workspace.choose": (a) =>
    `ワークスペースが複数あります。どれかを指定してください（${list(a, "names")}）`,
  "workspace.bad_name": (a) => `「${str(a, "value")}」はワークスペースの名前に使えません`,
  "workspace.bad_prefix": (a) => `「${str(a, "value")}」は ID の接頭辞に使えません`,
  "workspace.prefix_taken": (a) => `ID の接頭辞 ${str(a, "prefix")} はすでに使われています`,
  "workspace.not_empty": () =>
    "ワークスペースにまだタスクがあります。別のワークスペースへ移すか、完全に削除してください",
  "workspace.one_change": () =>
    "名前と接頭辞は同時に変えられません。どちらか一方を指定してください",

  // writes
  "patch.empty": () => "変更する項目がありません",
  "edit.changed": () => "編集している間にタスクが変わりました。開き直してください",
  "edit.invalid": (a, e) => withDetail(`${str(a, "field") || "入力"} を変更できません`, e),
  "move.invalid": (_, e) => withDetail("ここには移せません", e),
  "add.position": (_, e) => withDetail("この位置には作れません", e),
  "position.both": () => "前に置くか後ろに置くか、どちらか一方を指定してください",
  "dep.invalid": (_, e) => withDetail("ブロックの指定を変えられません", e),
  "status.unknown": (a) => `「${str(a, "name")}」というステータスはありません`,
  "schema.no_default": (a) => {
    const what = str(a, "what");
    const name = what === "kind" ? "種類" : what === "status" ? "ステータス" : "完了のステータス";
    return `スキーマに既定の${name}がありません。指定してください`;
  },
  "schema.violation": (a, e) => {
    const t = violations(a);
    return t ? `スキーマに合わない変更です: ${t}` : withDetail("スキーマに合わない変更です", e);
  },
  "ref.cross_store": (a) =>
    `${str(a, "store")} の ${str(a, "id")} は参照できません（参照は同じ Store の中だけです）`,
  "use.not_project": (a) => `${str(a, "id")} は ${str(a, "kind")} で、プロジェクトではありません`,

  // assignments (hh set)
  "assign.form": (a) => `「${str(a, "input")}」は key=value の形ではありません`,
  "assign.readonly": (a) => `${str(a, "key")} は変更できません`,
  "assign.unknown_key": (a) =>
    `${str(a, "key")} はフィールドでも、スキーマのプロパティでもありません`,
  "assign.single": (a) => `${str(a, "key")} には値を1つだけ指定できます`,
  "assign.missing_value": (a) => `${str(a, "key")} の値がありません`,
  "assign.bad_value": (a, e) =>
    withDetail(`${str(a, "key")} に「${str(a, "value")}」は指定できません`, e),

  // deleting and restoring
  "purge.changed": () => "確認のあとにタスクが変わりました。もう一度確かめてください",
  "purge.child_not_deleted": (a) =>
    `${str(a, "id")} が削除されていないため、完全には削除できません`,
  "purge.unconfirmed": (a) => {
    const t = a.targets;
    const n = Array.isArray(t) ? t.length : 0;
    return n > 0
      ? `${n} 件のタスクを完全に削除します。対象を確かめてください`
      : "完全に削除するには確認が必要です";
  },
  "restore_at.bad_time": (a) => `「${str(a, "at")}」は時刻でもコミットでもありません`,
  "restore.forward": (a) => `${str(a, "at")} の時点では、このタスクは別の Store へ移っていました`,
  "restore_at.not_found": (a) =>
    a.at
      ? `${str(a, "at")} の時点にはこのタスクがありませんでした`
      : "その時点にはこのタスクがありませんでした",
  "undo.nothing": () => "取り消せる操作がありません",
  "undo.blocked": () =>
    "ワークスペースにまだタスクがあるため取り消せません。別のワークスペースへ移すか、完全に削除してください",

  // comments
  "comment.not_found": (a) =>
    a.input ? `${str(a, "input")} というコメントは見つかりません` : "コメントが見つかりません",
  "comment.ambiguous": (a) => `${str(a, "input")} に当てはまるコメントが複数あります`,
  "comment.changed": () => "編集している間にコメントが変わりました。開き直してください",
  "comment.deleted": () => "このコメントは削除されています",
  "comment.no_author": () => "書いた人が分からないコメントは変更できません",
  "comment.not_author": (a) => `${str(a, "author")} が書いたコメントは変更できません`,
  "comment.empty": () => "コメントが空です",

  // stores and sync
  "store.read_only": (a) =>
    `${str(a, "store")} は読み取り専用です${a.reason ? `（${str(a, "reason")}）` : ""}`,
  "store.bad_name": (a) => `「${str(a, "value")}」は Store の名前に使えません`,
  "store.exists": (a) => `${str(a, "name")} という Store はすでにあります`,
  "store.not_found": (a) =>
    `${str(a, "name")} という Store はありません` +
    (list(a, "candidates") ? `（登録済み: ${list(a, "candidates")}）` : ""),
  "store.none": () => "Store がありません。hh store init で作ってください",
  "store.open": (a, e) => withDetail(`${str(a, "name")} を開けません`, e),
  "store.choose": (_, e) => withDetail("Store を選んでください", e),
  "store.conflict": (a) =>
    `${str(a, "named")} の ID ですが、指定された Store は ${str(a, "store")} です`,
  "store.locked": () => "別の hh が書き込み中です。少し待ってからやり直してください",
  "store.unpushed": (a) =>
    `${str(a, "name")} にはまだ push していない変更があります。先に同期してください`,
  "store.remote_required": () => "リモートの URL が必要です",
  "store.remote_not_empty": (_, e) => withDetail("リモートが空ではありません", e),
  "store.clone_failed": (a, e) => withDetail(`${str(a, "remote")} を取り込めません`, e),
  "store.not_hedgehug": (a) => `${str(a, "name")} は Hedgehug の Store ではありません`,
  "store.mount_failed": (a) => `追加できなかった Store があります（${list(a, "failed")}）`,
  "sync.dirty": () => "コミットしていない変更があるため同期できません",
  "sync.unrelated": () => "リモートの履歴がこの Store の履歴と無関係です",
  "sync.force_push": () =>
    "リモートの履歴が Hedgehug の記録なしに書き換えられています。hh doctor で確かめてください",
  "sync.background_needs_store": () => "バックグラウンドの同期には Store の指定が必要です",

  // accounts and GitHub
  "account.none": () =>
    "アカウントが設定されていません。hh auth login か config.yaml の accounts で足してください",
  "account.not_found": (a) =>
    `${str(a, "name")} というアカウントはありません` +
    (list(a, "candidates") ? `（登録済み: ${list(a, "candidates")}）` : ""),
  "account.exists": (a) => `アカウント ${str(a, "account")} はすでに設定されています`,
  "account.host_mismatch": (a) =>
    `アカウント ${str(a, "account")} のホストと、リモートのホストが違います`,
  "auth.token": (a) =>
    a.account
      ? `アカウント ${str(a, "account")} にトークンがありません。hh auth login で設定してください`
      : "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください",
  "auth.unauthorized": (a) =>
    `GitHub がアカウント ${str(a, "account")} のトークンを受け付けませんでした。有効か確かめてください`,
  "auth.insecure": (a) =>
    `${str(a, "url")} は http なので、トークンを送りません。https を使ってください`,
  "auth.host": (a) =>
    `アカウント ${str(a, "account")} のホストは ${str(a, "host")} です。指定と合いません`,
  "auth.need_token": (a) =>
    `アカウント ${str(a, "account")} はキーチェーンにトークンを持ちます。標準入力で渡してください`,
  "auth.device_only": (a) =>
    `アカウント ${str(a, "account")} はデバイスフローでログインします。標準入力のトークンは使えません`,
  "auth.name": () => "アカウントの名前が必要です",
  "github.forbidden": (a) =>
    `GitHub がアカウント ${str(a, "account")} の操作を許可しませんでした。権限を確かめてください`,
  "github.not_found": (a) =>
    `アカウント ${str(a, "account")} では GitHub のリポジトリが見つかりません`,
  "github.rate_limit": (a) =>
    `GitHub の利用制限に達しました（アカウント ${str(a, "account")}）。しばらく待ってください`,

  // queries, views, schema
  "query.invalid": (_, e) => withDetail("クエリが正しくありません", e),
  "query.tree_order": () => "ツリーは rank の順に並ぶため、group: と sort: は使えません",
  "log.workspace_with_task": () => "タスクの履歴にはワークスペースを指定できません",
  "view.bad_name": (a) => `「${str(a, "value")}」はビューの名前に使えません`,
  "view.not_found": (a) => `${str(a, "name")} というビューはありません`,
  "view.name_taken": (a) =>
    a.existing && a.existing !== a.name
      ? `ビュー ${str(a, "existing")} と名前が重複します（大文字小文字は区別しません）`
      : `${str(a, "name")} というビューはすでにあります`,
  "view.broken": (a) =>
    `ビュー ${str(a, "name")} のファイルが壊れています。直すか、削除してください`,
  "view.bad_value": (_, e) => withDetail("ビューの設定が正しくありません", e),
  "view.bad_column": (a) =>
    `列 ${str(a, "column")} は、フィールドでもスキーマのプロパティでもありません`,
  "view.tree_order": () => "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
  "migrate.invalid": (_, e) => withDetail("移行するとスキーマに合わないタスクがあります", e),

  // notifications
  "inbox.ulid": (a) => `${str(a, "input")} は完全な ULID ではありません`,
  "inbox.not_found": (a) => `${str(a, "input")} という通知は見つかりません`,
  "inbox.not_revertable": (_, e) => withDetail("この通知の操作は取り消せません", e),
  "notification.unread": () => "通知を未読には戻せません",

  // choices and confirmations (the CLI asks; the GUI picks first)
  "choice.required": (a) => `${str(a, "what")} を選んでください（${list(a, "candidates")}）`,
  "confirm.required": (a) => `${str(a, "what")} には確認が必要です`,

  // the server itself
  network: () => "サーバーに接続できません",
  "request.invalid": (_, e) => {
    const body = e.body as { errors?: { message?: string }[] } | undefined;
    const ms = (body?.errors ?? []).map((x) => x.message).filter(Boolean);
    return ms.length > 0
      ? `入力が正しくありません: ${ms.join(" / ")}`
      : withDetail("入力が正しくありません", e);
  },
  "host.refused": () => "この画面は 127.0.0.1 か localhost のアドレスからだけ開けます",
  "presence.unknown_client": () => "接続が切れています。再接続を待ってください",
  "http.404": () => "存在しない API です。画面と hh のバージョンが合っていないかもしれません",
  internal: (_, e) => withDetail("サーバーで予期しないエラーが起きました", e),
};

// errorMessage is the words for an error shown to the user.
export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    const make = messages[e.type];
    if (make) return make(e.args, e);
    return `操作に失敗しました（${e.type}）: ${e.detail}`;
  }
  const msg = e instanceof Error ? e.message : String(e);
  return `操作に失敗しました: ${msg}`;
}
