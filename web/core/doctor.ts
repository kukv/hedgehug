import type { Schemas } from "./api/client";
import { CATEGORY_LABEL } from "./categories";
import { repairedKnownText, type TaskName } from "./notifications";

type Finding = Schemas.Finding;
export type DoctorReport = Schemas.DoctorReport;
export type PurgePlan = Schemas.PurgePlan;

// The Japanese of the values of Finding.check (internal/app/usecase/maintenance: checkOrder, and the workspace check).
const CHECKS: Record<string, string> = {
  schema: "スキーマ",
  template: "テンプレート",
  prop_name: "プロパティ名",
  override: "プロジェクトの設定",
  automation: "自動化",
  task: "タスク",
  file: "ファイル",
  view: "ビュー",
  worktree: "作業ツリー",
  merge_driver: "merge driver",
  inbox: "通知",
  git: "git",
  cross_store_ref: "Store をまたぐ参照",
  duplicate_across_stores: "Store をまたぐ重複",
  forward_target: "移動先",
  unexpected_force_push: "想定外の force push",
  workspace: "ワークスペース",
};

/** The name of a check; a check that is not known is written as it is. */
function checkName(check: string): string {
  return CHECKS[check] ?? check;
}

type FindingGroup = { check: string; name: string; items: Finding[] };

/** The findings by check, in the order their checks first come (the server sorts them by check). */
export function groupFindings(findings: readonly Finding[]): FindingGroup[] {
  const groups = new Map<string, FindingGroup>();
  for (const f of findings) {
    let g = groups.get(f.check);
    if (!g) {
      g = { check: f.check, name: checkName(f.check), items: [] };
      groups.set(f.check, g);
    }
    g.items.push(f);
  }
  return [...groups.values()];
}

export type Severity = "error" | "warning" | "info";

/** The severity of a finding; the server writes an error as "error", and so does an empty one here. */
export function severityOf(f: Finding): Severity {
  return f.severity === "warning" || f.severity === "info" ? f.severity : "error";
}

const rec = (v: unknown): Record<string, unknown> =>
  typeof v === "object" && v !== null ? (v as Record<string, unknown>) : {};
const str = (v: unknown): string => (typeof v === "string" ? v : "");

/** What a repair of doctor --fix did (Fixed.kind and Fixed.value), in words. */
export function fixedText(
  kind: string,
  value: unknown,
  taskName?: TaskName,
  cut?: readonly { prop: string; value: string }[] | null,
): string {
  switch (kind) {
    case "id_collision": {
      const v = rec(value);
      const [old, now] = [str(v.old), str(v.new)];
      return old && now
        ? `ID が重複していたため、${old} から ${now} に振り直しました`
        : "ID が重複していたため、振り直しました";
    }
    case "id_renumbered":
      return typeof value === "string" && value
        ? `ID を振り直しました（元の ID: ${value}）`
        : "ID を振り直しました";
    case "id_prefix":
      return "ID の接頭辞がワークスペースと違っていたため、振り直しました";
    case "workspace_mismatch":
      return "親と違うワークスペースにあったため、親のワークスペースに合わせました";
    case "workspace_revived":
      return "削除されていたワークスペースを復元しました";
    case "legacy_field": {
      const names = Array.isArray(value) ? value.filter((n) => typeof n === "string") : [];
      const moved = names.length
        ? `props の ${names.join("・")} をタスクの項目へ移しました`
        : "props に残っていた項目をタスクの項目へ移しました";
      // A date made of a time says what it lost, with the value as it was written.
      const lost = (cut ?? []).map((c) => `${c.prop} の時刻を落としました: ${c.value}`);
      return lost.length ? `${moved}（${lost.join("、")}）` : moved;
    }
  }
  // The kinds a sync repairs too are told as the inbox tells them.
  return repairedKnownText({ kind, value }, taskName) ?? kind;
}

// hh purge takes at most this many days (internal/app/usecase/purge.go).
const MAX_PURGE_DAYS = 36500;

/** The days of "N 日より古いもの": a whole number from 0, or null when the input is not one. */
export function parsePurgeDays(input: string): number | null {
  const s = input.trim().normalize("NFKC");
  if (!/^[0-9]+$/.test(s)) return null;
  const n = Number(s);
  return n <= MAX_PURGE_DAYS ? n : null;
}

// The words of a finding come from its code and args (usecase.Finding): the server writes its msg and
// hint in English for the CLI. A finding whose code is not known here is shown as the server wrote it.
type Words = (a: readonly string[], f: Finding) => string;

// The list of values a schema finding is about: the priorities or the labels, from where it is.
const listOf = (f: Finding): string | undefined =>
  f.where.endsWith(": priorities") ? "優先度" : f.where.endsWith(": labels") ? "ラベル" : undefined;

const cat = (c: string | undefined): string =>
  (CATEGORY_LABEL as Record<string, string | undefined>)[c ?? ""] ?? c ?? "";
const plan = (what: string | undefined): string =>
  what === "sprint" ? "スプリント" : what === "release" ? "リリース" : (what ?? "");

// The problems of a task (index.Problem.Code), as the canvas words them.
const TASK_PROBLEMS: Record<string, string> = {
  unknown_kind: "未定義の種類です",
  unknown_status: "未定義のステータスです",
  missing_title: "タイトルがありません",
  bad_value: "値が正しくありません",
  placement: "この親には置けません",
  transition: "このステータスへは遷移できません",
  required: "必須の項目が空です",
  missing_ref: "参照先がありません",
  cycle: "循環しています",
  broken: "ファイルを読み込めません",
  no_id: "ID がありません",
  duplicate_id: "ID が重複しています",
  id_prefix: "ID の接頭辞がワークスペースと違います",
  parent_deleted: "親が削除されています",
  no_workspace: "ワークスペースがないか、この Store のワークスペースではありません",
  workspace_mismatch: "親と違うワークスペースにあります",
  legacy_field:
    "古い形の項目が props に残っています（関連と、優先度・開始日・期限・見積もり・ポイントは、タスクの項目です）",
};

const MESSAGES: Record<string, Words> = {
  "store.old_format": ([from, to]) =>
    `Store の形式が古いです（${from}、最新は ${to}）。hh store convert で新しい形式にしてください`,
  "schema.name_empty": (_, f) => (listOf(f) ? "値が空です" : "名前が空です"),
  "schema.value_twice": ([v], f) =>
    `${listOf(f) ?? "リスト"}に「${v}」が 2 回定義されています（大文字小文字は区別しません）`,
  "schema.status_twice": ([n]) => `ステータス「${n}」が 2 回定義されています`,
  "schema.unknown_category": ([c]) => `「${c}」というカテゴリはありません`,
  "schema.default_status": ([c, n]) =>
    `カテゴリ${cat(c)}の既定のステータスは 1 つだけにしてください（今は ${n} 個）`,
  "schema.bad_template_name": ([e]) => `テンプレートの名前が正しくありません: ${e}`,
  "schema.templates_only_on_store": () =>
    "templates.only はプロジェクトの上書き（scoped の templates.only）にだけ使えます。Store 全体では効かないので消してください",
  "schema.unknown_kind": ([k]) => `「${k}」という種類はありません`,
  "schema.prop_builtin": ([n]) =>
    `「${n}」は組み込みのフィールドなので、プロパティの名前に使えません`,
  "schema.prop_twice": ([n]) => `プロパティ「${n}」が 2 回定義されています`,
  "schema.unknown_type": ([t]) =>
    t === "task_ref"
      ? "関連は固定です。hh dep か詳細の関連の欄で付けます（task_ref 型は使えません）"
      : `「${t}」という型はありません`,
  "schema.values_empty": (_, f) =>
    listOf(f) === "優先度" ? "priorities が空です。優先度は 1 つ以上必要です" : "values が空です",
  "schema.unknown_status": ([s]) => `「${s}」というステータスはありません`,
  "schema.not_requirable": ([n]) =>
    `「${n}」はプロパティでも、必須にできるフィールドでもありません`,
  "schema.bad_where": ([e]) => `where が正しくありません: ${e}`,
  "schema.where_twice": ([w]) => `${w} に上書きが 2 つあります`,
  "schema.default_hidden": ([s, c]) =>
    `「${s}」はカテゴリ${cat(c)}の既定なので、非表示にできません`,
  "schema.unreadable": ([e]) => `schema.yaml を読み込めません: ${e}`,
  "schema.tasks_skipped": () =>
    "schema.yaml を読み込めないため、タスクとスキーマを照合していません",

  "automation.where_form": () => "where は project:<ULID> の形で書いてください",
  "automation.bad_ulid": ([e]) => `ULID が正しくありません: ${e}`,
  "automation.not_project": ([w]) => `${w} はこの Store のプロジェクトではないか、削除されています`,
  "automation.id_empty": () => "id が空です",
  "automation.id_space": ([id]) =>
    `id「${id}」に空白か改行があります。コミットの trailer には書けません`,
  "automation.id_twice": ([id]) => `id「${id}」が 2 回使われています`,
  "automation.not_one_of": ([v, want]) => `「${v}」は使えません（${want} のどれかです）`,
  "automation.set_both": () => "set には status か category のどちらか一方だけを書いてください",
  "automation.set_none": () => "set に status か category を書いてください",
  "automation.unknown_status": ([s]) => `ステータス「${s}」はないか、プロジェクトで非表示です`,
  "automation.unknown_category": ([c]) => `「${c}」というカテゴリはありません`,
  "automation.where_twice": ([w]) => `${w} が 2 回使われています`,
  "automation.no_rule": ([id]) => `id「${id}」のルールはありません`,
  "automation.unreadable": ([e]) => `automations.yaml を読み込めません: ${e}`,

  "prop_name.query_key": () => "この名前のプロパティでは、クエリで絞り込めません",
  "override.not_project": ([w]) => `${w} はこの Store のプロジェクトではないか、削除されています`,
  "template.bad_name": ([n, e]) => `テンプレートの名前「${n}」が正しくありません: ${e}`,
  "template.missing": ([n]) => `テンプレート「${n}」がありません`,
  "template.bad_label": ([n, path, l]) =>
    `テンプレート「${n}」の ${path}: ラベル「${l}」が設定の「ラベル」のリストにありません`,

  "task.plan_other_workspace": ([field, what, ulid]) =>
    `${field}: ${plan(what)} ${ulid} はこのタスクのワークスペースにありません`,
  "task.plan_missing": ([field, what, ulid]) =>
    `${field}: ${plan(what)} ${ulid} はこの Store にありません`,

  "file.bad_name": () => "ファイル名が命名規則に合いません",
  "file.undecodable": ([e]) => `ファイルを読み込めません: ${e}`,
  "file.unreadable": ([e]) => `ファイルを読み込めません: ${e}`,
  "file.ulid_mismatch": ([u]) => `ファイルの ULID（${u}）がパスの ULID と違います`,
  "file.no_task": ([t]) => `タスク ${t} のファイルがありません`,
  "file.missing_field": ([n]) => `${n} がありません`,
  "file.bad_value": ([e]) => `値が正しくありません: ${e}`,
  "file.not_view": () => "ビューではありません（views/ には <名前>.yaml だけを置きます）",
  "file.bad_view_name": ([e]) => `ビューの名前が正しくありません: ${e}`,
  "numbers.bad_prefix": ([p, e]) => `接頭辞「${p}」が正しくありません: ${e}`,
  "numbers.below_one": ([p, n]) => `接頭辞「${p}」の番号 ${n} が 1 より小さくなっています`,
  "store_meta.unreadable": ([e]) =>
    `store.yaml を読み込めないため、タスクとスキーマを照合していません: ${e}`,

  "git.open": ([e]) => `git のリポジトリを開けません: ${e}`,
  "git.status": ([e]) => `作業ツリーの状態を読み込めません: ${e}`,
  "git.object": ([e]) => `git のオブジェクトを読み込めません: ${e}`,
  "git.open_store": ([e]) => `Store を開けません: ${e}`,
  "git.index": ([e]) => `インデックスを作成できません: ${e}`,
  "worktree.staged": () => "ステージした変更があります。次の操作のコミットに入ります",
  "worktree.unstaged": () => "コミットしていない変更があります",
  "merge_driver.config": ([e]) => `.git/config を読み込めません: ${e}`,
  "merge_driver.unregistered": () =>
    "merge=hedgehug を指定したファイルがありますが、merge driver が .git/config に登録されていません",
  "merge_driver.not_on_path": () =>
    "hedgehug が PATH にないため、git が merge driver を実行できません",
  "inbox.unreadable": ([e]) => `merge driver の記録を読み込めません: ${e}`,
  "inbox.undecodable": ([e]) =>
    `merge driver の記録を読み込めないため、通知に取り込まずに残しています: ${e}`,
  "inbox.waiting": ([n]) => `通知にまだ取り込んでいない merge driver の記録が ${n} 件あります`,

  "workspace.same_prefix": ([v, name, ulid]) =>
    `ワークスペース ${name}（${ulid}）と同じ接頭辞 ${v} です`,
  "workspace.same_name": ([v, name, ulid]) =>
    `ワークスペース ${name}（${ulid}）と同じ名前 ${v} です`,
  "view.same_name": ([rel]) => `${rel} と、大文字小文字の違いを除いて同じ名前です`,
  "view.problem": ([p]) => `ビューの設定に問題があります: ${p}`,
  "view.unknown_column": ([c]) =>
    `列「${c}」はフィールドでもスキーマのプロパティでもないため、画面には出ません`,
  "view.bad_gantt_prop": ([p]) =>
    `ガントのプロパティ「${p}」はスキーマの日付ではないため、バーを描けません`,
  "plan.no_workspace": () => "ワークスペースがありません",
  "plan.unknown_workspace": ([w]) => `ワークスペース ${w} はこの Store にありません`,
  "plan.same_name": ([n, rel]) => `同じワークスペースの ${rel} と同じ名前「${n}」です`,

  "rebuild.unfinished": ([e]) => `履歴の書き換えのあとの作り直しが終わっていません: ${e}`,
  "force_push.unexpected": () =>
    "前回の同期で、origin の履歴が手元と食い違っていました（想定外の force push）",
  "force_push.rebuild_failed": ([e]) =>
    `origin の履歴が手元と食い違っていますが、Store を作り直せません: ${e}`,
  cross_store_ref: ([field, store, id]) =>
    `${field} が ${store} の ${id} を指しています。参照できるのは同じ Store の中だけです`,
  duplicate_across_stores: (a) =>
    `同じタスクが ${a.join("、")} にあります。いらないほうを hh rm と hh purge --older-than 0d --store <store> で消してください`,
  "forward.unmounted": ([u]) => `マウントしていない Store（ULID ${u}）へ移動しています`,
};

const restore = (f: Finding) => `手で直してコミットするか、git restore ${f.where} で戻してください`;
const rerun = () => "原因を直してから、もう一度 hh doctor --fix を実行してください";

const HINTS: Record<string, Words> = {
  "schema.unreadable": () => "git restore schema.yaml で戻すか、手で直してコミットしてください",
  "automation.unreadable": (_, f) =>
    `git restore ${f.where} で戻すか、手で直してコミットしてください`,
  "prop_name.query_key": () => "hh schema migrate prop で名前を変更してください",
  "override.not_project": () => "プロジェクトを復元するか、上書きを外してください",
  "template.bad_name": ([n]) => `templates/${n}.md を追加するか、名前を変更してください`,
  "template.missing": ([n]) => `templates/${n}.md を追加するか、名前を変更してください`,
  "template.bad_label": ([n]) =>
    `設定の「ラベル」に足すか（hh schema labels add）、hh template edit ${n} でテンプレートを直してください`,
  "task.duplicate_id": () =>
    "hh doctor --fix で直せます。hh sync でも、push 済みのタスクを残して振り直します",
  "task.id_prefix": () => "hh doctor --fix で直せます",
  "task.workspace_mismatch": () => "hh doctor --fix で直せます",
  "task.cycle": () => "hh doctor --fix で直せます",
  "task.missing_ref": () => "hh doctor --fix で直せます",
  "task.parent_deleted": () => "hh doctor --fix で直せます",
  // args: the field, the message, then why --fix leaves the task (conflict, bad_value; "" when it
  // moves it) and the names of the relations or built-in properties that keep it.
  "task.legacy_field": ([, , reason, ...names]) => {
    const list = names.join("・");
    if (reason === "conflict")
      return `${list} は移し先の項目にすでに値があり、どちらが新しいか分からないため、--fix では移せません。hh edit で決めてください`;
    if (reason === "bad_value")
      return `${list} に、移し先の項目の型に合わない値があるため、--fix では移せません。hh edit で正しい値に直してください`;
    return "hh doctor --fix でタスクの項目へ移せます";
  },
  "task.plan_other_workspace": () => "hh edit で外すか、タスクを元のワークスペースに戻してください",
  "task.plan_missing": () => "hh edit で外してください",
  "file.bad_name": (_, f) => restore(f),
  "file.undecodable": (_, f) => restore(f),
  "file.unreadable": (_, f) => restore(f),
  "file.ulid_mismatch": (_, f) => restore(f),
  "file.no_task": (_, f) => restore(f),
  "file.missing_field": (_, f) => restore(f),
  "file.bad_value": (_, f) => restore(f),
  "file.not_view": (_, f) => restore(f),
  "file.bad_view_name": (_, f) => restore(f),
  "numbers.bad_prefix": (_, f) => restore(f),
  "numbers.below_one": (_, f) => restore(f),
  "view.problem": (_, f) => restore(f),
  "view.same_name": (_, f) => restore(f),
  "plan.no_workspace": (_, f) => restore(f),
  "plan.unknown_workspace": (_, f) => restore(f),
  "plan.same_name": () => "どちらかの名前を手で変更してコミットしてください",
  "store_meta.unreadable": (_, f) =>
    `git restore ${f.where} で戻すか、手で直してコミットしてください`,
  "worktree.staged": ([p]) => `git restore --staged ${p}`,
  "worktree.unstaged": ([p]) => `git restore ${p}。新しいファイルなら消してください`,
  "merge_driver.unregistered": () => "hh sync を一度実行してください",
  "merge_driver.not_on_path": () => "hedgehug を PATH に置いてください",
  "inbox.undecodable": () => "ファイルを消してください",
  "inbox.waiting": () => "hh inbox を実行してください",
  "git.index": () =>
    "コミット済みの schema.yaml が壊れているかもしれません。正しいものに戻してコミットしてください",
  "workspace.same_prefix": () => "hh workspace set-prefix（か rename）でどちらかを変更してください",
  "workspace.same_name": () => "hh workspace set-prefix（か rename）でどちらかを変更してください",
  "view.unknown_column": () => "hh view save --columns で列を変更してください",
  "view.bad_gantt_prop": () => "hh view save --gantt-start と --gantt-end で変更してください",
  "rebuild.unfinished": rerun,
  "force_push.rebuild_failed": rerun,
  "force_push.unexpected": () =>
    "hh doctor --fix で、origin の履歴の上に Store を作り直し、未 push の変更を再適用します。再適用できない変更があれば、古い Store を残します",
};

/** What a finding says, in Japanese when its code is known. */
export function findingText(f: Finding): string {
  const code = f.code ?? "";
  const args = f.args ?? [];
  const words = MESSAGES[code];
  if (words) return words(args, f);
  const problem = code.startsWith("task.") ? TASK_PROBLEMS[code.slice("task.".length)] : undefined;
  if (problem) return args[0] ? `${args[0]}: ${problem}` : problem;
  return f.msg;
}

/** How to fix a finding, in Japanese when its code is known; "" when the server gives no advice. */
export function findingHint(f: Finding): string {
  if (!f.hint) return "";
  const words = HINTS[f.code ?? ""];
  return words ? words(f.args ?? [], f) : f.hint;
}
