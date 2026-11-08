import { describe, expect, it } from "vitest";
import { ApiError } from "./api/client";
import { errorMessage } from "./messages";

const err = (
  type: string,
  args: Record<string, unknown> = {},
  detail = "english",
  body?: unknown,
) => new ApiError({ status: 400, type, detail, args, body });

describe("errorMessage", () => {
  it("puts the args into the words", () => {
    expect(errorMessage(err("task.not_found", { input: "REL-9" }))).toBe(
      "REL-9 というタスクは見つかりません",
    );
    expect(errorMessage(err("comment.not_author", { author: "alice" }))).toBe(
      "alice が書いたコメントは変更できません",
    );
    expect(errorMessage(err("assign.bad_value", { key: "points", value: "x" }, ""))).toBe(
      "points に「x」は指定できません",
    );
    expect(errorMessage(err("store.not_found", { name: "foo", candidates: ["a", "b"] }))).toBe(
      "foo という Store はありません（登録済み: a、b）",
    );
    expect(
      errorMessage(err("store.bad_color", { value: "red", candidates: ["teal", "orange"] })),
    ).toBe("「red」は Store の色に使えません（teal、orange）");
  });

  it("words restore_at.not_found for a restore and for reading a point in time", () => {
    expect(errorMessage(err("restore_at.not_found", { at: "abc123" }))).toBe(
      "abc123 の時点にはこのタスクがありませんでした",
    );
    expect(errorMessage(err("restore_at.not_found"))).toBe(
      "その時点にはこのタスクがありませんでした",
    );
  });

  it("words task.moved with and without the store and the new ID", () => {
    expect(errorMessage(err("task.moved", { input: "REL-9", store: "work", id: "REL-10" }))).toBe(
      "REL-9 は work へ移りました（今の ID は REL-10）",
    );
    expect(errorMessage(err("task.moved", { input: "REL-9", store: "work" }))).toBe(
      "REL-9 は work へ移りました",
    );
    expect(errorMessage(err("task.moved", { input: "REL-9" }))).toBe(
      "REL-9 は別の Store へ移りました",
    );
  });

  it("words workspace.not_empty with the number of tasks, deleted ones included", () => {
    expect(errorMessage(err("workspace.not_empty", { tasks: 3 }))).toBe(
      "ワークスペースにタスクが 3 件（削除済みを含む）残っています。別のワークスペースへ移すか、削除したうえで hh purge --older-than 0d で消してください",
    );
    expect(errorMessage(err("workspace.not_empty", { tasks: 0 }))).toBe(
      "ワークスペースにタスクが 0 件（削除済みを含む）残っています。別のワークスペースへ移すか、削除したうえで hh purge --older-than 0d で消してください",
    );
    expect(errorMessage(err("workspace.not_empty", { tasks: "x" }))).toBe(
      "ワークスペースにまだタスクがあります（削除済みを含む）。別のワークスペースへ移すか、削除したうえで hh purge --older-than 0d で消してください",
    );
    expect(errorMessage(err("workspace.not_empty"))).toBe(
      "ワークスペースにまだタスクがあります（削除済みを含む）。別のワークスペースへ移すか、削除したうえで hh purge --older-than 0d で消してください",
    );
  });

  it("words restore.forward with the point in time", () => {
    expect(errorMessage(err("restore.forward", { at: "abc123" }))).toBe(
      "abc123 の時点では、このタスクは別の Store へ移っていました",
    );
  });

  it("words purge and the bulk read of notifications", () => {
    expect(errorMessage(err("purge.changed"))).toBe(
      "確認のあとにタスクが変わりました。もう一度確認してください",
    );
    expect(errorMessage(err("purge.bad_period", { value: "abc" }))).toBe(
      "「abc」は期間ではありません。90d のように日数で書いてください（0d はすべて）",
    );
    expect(errorMessage(err("notification.read_args"))).toBe(
      "すべてを既読にするか、通知を指定してください（両方は指定できません）",
    );
  });

  it("words the codes of a move to another store", () => {
    expect(errorMessage(err("move.changed"))).toBe(
      "確認のあと、移動するタスクが変わりました。移動の内容を読み込み直して、確認してから移動してください",
    );
    expect(errorMessage(err("move.same_store", { store: "org-a" }))).toBe(
      "タスクはすでに org-a にあります。同じ Store の中で移すには、別のワークスペースを選んでください",
    );
    expect(errorMessage(err("move.store_required"))).toBe("移動先の Store を選んでください");
    expect(errorMessage(err("move.exists", { store: "org-b", ulid: "01J" }))).toBe(
      "01J はすでに org-b にあります",
    );
    expect(errorMessage(err("move.unfinished", { store: "org-b" }))).toBe(
      "org-b へは移しましたが、移動元の片付けが終わっていません。もう一度同じ移動をすると完了します",
    );
    // the violations of move.invalid are for the dialog to list; the words
    // keep the English detail
    expect(
      errorMessage(
        err("move.invalid", { store: "org-b", violations: ["  API-1 x: bad"] }, "org-b refuses"),
      ),
    ).toBe("ここには移せません: org-b refuses");
  });

  it("says that automations.yaml stopped a move", () => {
    expect(errorMessage(err("automation.invalid", { store: "org-a" }, "boom"))).toBe(
      "automations.yaml に問題があります: boom",
    );
  });

  it("asks for the choices of a move by what they are", () => {
    expect(
      errorMessage(err("choice.required", { what: "--workspace", candidates: ["a", "b"] })),
    ).toBe("移動先のワークスペースを選んでください（a、b）");
    expect(
      errorMessage(err("choice.required", { what: "--scoped", projects: ["API-2 P", "API-9 Q"] })),
    ).toBe(
      "プロジェクト（API-2 P、API-9 Q）の設定（表示と自動化）を持っていくか、持っていかないかを選んでください",
    );
    expect(
      errorMessage(err("choice.required", { what: "--account", candidates: ["alice", "bob"] })),
    ).toBe("アカウントを選んでください（alice、bob）");
  });

  it("words the writes of stores and the choice of an account", () => {
    expect(errorMessage(err("store.set_args"))).toBe("色か表示のどちらかを指定してください");
    expect(
      errorMessage(err("choice.required", { what: "--account", candidates: ["personal", "work"] })),
    ).toBe("アカウントを選んでください（personal、work）");
    expect(errorMessage(err("store.unpushed", { name: "org-a" }))).toBe(
      "org-a にはまだ push していない変更があります。先に同期してください",
    );
    expect(errorMessage(err("store.mount_failed", { failed: ["a", "b"] }))).toBe(
      "追加できなかった Store があります（a、b）",
    );
  });

  it("words undo.blocked by the operation", () => {
    expect(errorMessage(err("undo.blocked", { op: "purge" }))).toBe(
      "完全に消したものは取り消せません",
    );
    for (const op of ["move-in", "move-out"]) {
      expect(errorMessage(err("undo.blocked", { op }))).toBe(
        "Store 間の移動は取り消せません。hh mv --store で元の Store へ戻してください",
      );
    }
    expect(errorMessage(err("undo.blocked"))).toContain("ワークスペースにまだタスクがあるため");
  });

  it("words sync.force_push", () => {
    expect(errorMessage(err("sync.force_push"))).toBe(
      "remote の履歴が手元と食い違っています。確認してから、hh doctor --fix で origin の新しい履歴にそろえてください",
    );
  });

  it("tells the two auth.token apart", () => {
    expect(errorMessage(err("auth.token", { account: "work" }))).toContain("work");
    expect(errorMessage(err("auth.token"))).toBe(
      "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください",
    );
  });

  it("words the codes of the accounts and the device flow", () => {
    expect(errorMessage(err("auth.device_running"))).toBe("Device Flow はすでに実行中です");
    expect(errorMessage(err("auth.device_not_found"))).toBe(
      "その Device Flow はすでに終わっているか、ありません",
    );
    expect(errorMessage(err("auth.canceled"))).toBe("ログインをやめました");
    expect(errorMessage(err("request.canceled"))).toBe(
      "リクエストが途中で中断されました。もう一度試してください",
    );
    expect(errorMessage(err("auth.device_failed", {}, "the login was denied"))).toBe(
      "Device Flow でログインできませんでした: the login was denied",
    );
    expect(errorMessage(err("auth.client_id", { host: "ghe.example.com" }))).toBe(
      "ghe.example.com の Device Flow には、config.yaml の hosts[].client_id が要ります。トークンを貼るか、gh か環境変数を使ってください",
    );
    expect(
      errorMessage(err("auth.no_device_flow", { account: "work", uses: "env:GH_TOKEN" })),
    ).toBe(
      "アカウント work は env:GH_TOKEN からトークンを得るため、Hedgehug でのログインは不要です",
    );
    expect(
      errorMessage(err("auth.mismatch", { account: "work", host: "github.com", login: "bob" })),
    ).toBe("トークンは github.com/bob のもので、アカウント work のものではありません");
    expect(errorMessage(err("auth.token_empty"))).toBe("トークンが空です");
    expect(errorMessage(err("auth.login"))).toBe("アカウントのログイン名が必要です");
  });

  it("says which sources of a token the account call takes", () => {
    expect(errorMessage(err("auth.token_source", { token: "keychain" }))).toBe(
      "トークンの取得元「keychain」はここでは指定できません。gh か env:<VAR> を指定してください（トークンを貼る、Device Flow は別の操作です）",
    );
  });

  it("words the codes of the rules and repo_accounts", () => {
    expect(errorMessage(err("automation.exists"))).toBe("その id のルールはすでにあります");
    expect(errorMessage(err("automation.not_found"))).toBe("そのルールはもうありません");
    expect(errorMessage(err("automation.not_project", { id: "REL-2" }))).toBe(
      "REL-2 はプロジェクトではありません",
    );
    expect(errorMessage(err("repo_accounts.invalid", {}, "bad pattern"))).toBe(
      "ルールに問題があります: bad pattern",
    );
    expect(errorMessage(err("issue.repo_required"))).toBe(
      "リポジトリを org/repo の形で指定してください",
    );
  });

  it("lists what a schema violation broke", () => {
    const m = errorMessage(
      err("schema.violation", {
        violations: [
          { code: "placement", field: "parent", message: "a epic cannot be placed under task" },
        ],
      }),
    );
    expect(m).toContain("この親には置けません（parent）");
  });

  it("words the errors of a schema edit", () => {
    expect(errorMessage(err("schema.in_use", { count: 3, deleted: 0 }))).toBe(
      "タスク 3 件が使っているため削除できません。マージ… で別の定義へ移してから削除してください",
    );
    expect(errorMessage(err("schema.in_use", { count: 3, deleted: 1 }))).toBe(
      "タスク 3 件（うち削除済み 1 件）が使っているため削除できません。マージ… で別の定義へ移してから削除してください",
    );
    expect(errorMessage(err("schema.tasks_invalid", { violations: ["WEB-2: x"] }))).toBe(
      "変更すると規則に合わなくなるタスクがあります",
    );
    expect(errorMessage(err("schema.invalid", { problems: ["a", "b"] }))).toBe(
      "スキーマの規則に合わなくなります: a、b",
    );
    expect(errorMessage(err("schema.invalid", {}, "boom"))).toBe(
      "スキーマの規則に合わなくなります: boom",
    );
    expect(errorMessage(err("schema.parents_empty", { kinds: ["sub", "leaf"] }))).toBe(
      "種類 sub、leaf の置ける親がなくなるため削除できません。先に sub、leaf の置ける親を変えてください",
    );
    expect(errorMessage(err("schema.not_defined"))).toBe(
      "定義が見つかりません。画面を読み込み直してください",
    );
    expect(errorMessage(err("schema.order_stale"))).toBe(
      "ほかの変更と重なったため並べ替えられませんでした。読み込み直したので、もう一度並べ替えてください",
    );
    expect(errorMessage(err("schema.default_container"))).toBe(
      "既定にできるのは work の役割の種類だけです",
    );
    expect(errorMessage(err("schema.not_task_ref"))).toBe(
      "複数・逆方向・双方向は関連だけの設定です",
    );
    expect(errorMessage(err("schema.not_enum"))).toBe(
      "値を持てるのは enum と multi_enum のプロパティだけです",
    );
    expect(errorMessage(err("schema.value_invalid"))).toBe("値が空か、すでにあります");
  });

  it("words the errors of an override of a project", () => {
    expect(errorMessage(err("scoped.not_container", { id: "ME-2", kind: "task" }))).toBe(
      "ME-2 は task で、プロジェクトやエピックのようなコンテナではないため、上書きを設定できません",
    );
    expect(
      errorMessage(
        err("scoped.invalid", {
          problems: ['scoped[0].statuses: "done" is the default of done and cannot be hidden'],
        }),
      ),
    ).toBe(
      '上書きがスキーマに合いません: scoped[0].statuses: "done" is the default of done and cannot be hidden',
    );
    expect(errorMessage(err("scoped.invalid", {}, "boom"))).toBe(
      "上書きがスキーマに合いません: boom",
    );
  });

  it("joins the details of request.invalid", () => {
    const body = { errors: [{ message: "expected string" }, { message: "too short" }] };
    expect(errorMessage(err("request.invalid", {}, "validation failed", body))).toBe(
      "入力が正しくありません: expected string / too short",
    );
  });

  it("words the errors of a position given to an add", () => {
    expect(errorMessage(err("add.position", {}, "ME-3 is not a sibling"))).toBe(
      "この位置には作れません: ME-3 is not a sibling",
    );
    expect(errorMessage(err("position.both"))).toBe(
      "前に置くか後ろに置くか、どちらか一方を指定してください",
    );
  });

  it("shows the type and the detail of an unknown type", () => {
    expect(errorMessage(err("weird.thing", {}, "boom"))).toBe(
      "操作に失敗しました（weird.thing）: boom",
    );
  });

  it("words an error that is not an ApiError", () => {
    expect(errorMessage(new TypeError("Failed to fetch"))).toBe(
      "操作に失敗しました: Failed to fetch",
    );
    expect(errorMessage("x")).toBe("操作に失敗しました: x");
  });

  it("says in Japanese that the server cannot be reached", () => {
    expect(
      errorMessage(new ApiError({ status: 0, type: "network", detail: "Failed to fetch" })),
    ).toBe("サーバーに接続できません");
  });
});

describe("store.read_only", () => {
  it("words the three reasons in Japanese and leaves out the brackets without one", () => {
    const m = (reason?: string) =>
      errorMessage(err("store.read_only", { store: "shared", reason }));
    expect(m("the GitHub permission is read")).toBe(
      "shared は閲覧のみです（GitHub の権限が読み取りのみです）",
    );
    expect(m("the remote cannot be written")).toBe(
      "shared は閲覧のみです（remote に書き込めません）",
    );
    expect(m("the remote denied the push")).toBe(
      "shared は閲覧のみです（remote が push を拒否しました）",
    );
    expect(m()).toBe("shared は閲覧のみです");
  });
});
