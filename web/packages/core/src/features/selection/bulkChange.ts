import type {
  BulkChangeInput,
  BulkChangeResult,
  BulkChoices,
  BulkField,
} from "../../model/bulkChange";
import { Refused } from "../../model/errors";
import type { ItemRef } from "../../model/itemAction";
import type { ItemActionsPort } from "../../port/itemActions";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Detail } from "../items/detail";
import { sprintSaveFailed } from "../planning/sprintDialogs";
import type { Toasts } from "../toasts/toasts";
import {
  type BulkOption,
  confirmLines,
  inputOf,
  summaryText,
  valueOptions,
} from "./bulkChangeForm";
import { bulkChangeToast } from "./bulkChangeResult";
import type { RowSelection, SelectedRow } from "./rowSelection";

// 一覧で選んだ行の一括変更(F2b2)。ダイアログの状態・予告・確認・実行・結果のトーストまでの流れ。

export interface BulkChangeForm {
  stage: "form";
  summary: string;
  field: BulkField;
  options: BulkOption[];
  // option は、セレクトで選んでいる値。values は、関連システムで選んだ名前。text は、数・日付の入力。
  option: string;
  values: string[];
  text: string;
  error: string | null;
  busy: boolean;
}

export interface BulkChangeConfirm {
  stage: "confirm";
  title: string;
  lines: string[];
  ok: string;
}

export interface BulkChangeDeps {
  actions: Pick<ItemActionsPort, "bulkChangeChoices" | "bulkChangePreview" | "bulkChange">;
  toasts: Toasts;
  detail: Pick<Detail, "reload">;
}

export interface BulkChangeOpen {
  // after は、変更を試みたあと(成功も失敗も)に、画面を読み直すために呼ぶ。
  after: () => Promise<void>;
  // suffix は、変わったものがあるときのトーストの末尾(バックログの画面は offBacklogText を渡す)。
  suffix?: (r: BulkChangeResult) => string;
}

export interface BulkChange {
  state: Readable<BulkChangeForm | BulkChangeConfirm | null>;
  // open は、その時点の選択の行を対象に固定し、選択肢を読んでからダイアログを出す。
  open(selection: RowSelection, o: BulkChangeOpen): Promise<void>;
  setField(f: BulkField): void;
  setOption(v: string): void;
  toggleValue(name: string): void;
  setText(t: string): void;
  // submit は、予告を読んで確認を出す。変わるものがなければ、確認なしで結果を知らせて閉じる。
  submit(): Promise<void>;
  confirm(): Promise<void>;
  cancel(): void;
}

const isSystems = (f: BulkField) => f === "systemsAdd" || f === "systemsRemove";
const isTyped = (f: BulkField) =>
  f === "points" || f === "estimate" || f === "start" || f === "end";

// canSubmit は、実行ボタンを押せるか。関連システムは 1 つ以上選んでいる、選択肢のある項目は選択肢がある。
export function canSubmit(f: BulkChangeForm): boolean {
  if (isSystems(f.field)) return f.values.length > 0;
  if (isTyped(f.field)) return true;
  return f.options.length > 0;
}

interface Opened {
  rows: SelectedRow[];
  choices: BulkChoices;
  o: BulkChangeOpen;
  input: BulkChangeInput | null;
}

const ref = (r: SelectedRow): ItemRef => ({ storeId: r.storeId, itemId: r.itemId });

export function createBulkChange(deps: BulkChangeDeps): BulkChange {
  const state = writable<BulkChangeForm | BulkChangeConfirm | null>(null);
  let opened: Opened | null = null;
  let form: BulkChangeForm | null = null;
  // seq は、開く・閉じるの世代。待っているあいだに閉じた・開き直したら、古い結果を捨てる。
  let seq = 0;
  // busy は、変更の実行中。そのあいだの confirm は受けない。
  let busy = false;

  const put = (patch: Partial<BulkChangeForm>) => {
    if (form === null || get(state)?.stage !== "form") return;
    form = { ...form, ...patch };
    state.set(form);
  };

  const close = () => {
    opened = null;
    form = null;
    state.set(null);
  };

  const inputFor = (f: BulkChangeForm): BulkChangeInput => {
    if (isSystems(f.field)) return { field: f.field, value: "", values: f.values, category: "" };
    if (isTyped(f.field)) return { field: f.field, value: f.text, values: [], category: "" };
    return inputOf(f.field, f.option);
  };

  const notify = (o: Opened, r: BulkChangeResult) => {
    const suffix = o.o.suffix ? o.o.suffix(r) : "";
    const t = bulkChangeToast(r, o.rows, suffix);
    deps.toasts.show(t.text, { error: t.error });
  };

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : sprintSaveFailed, { error: true });

  return {
    state: readonly(state),
    async open(selection, o) {
      const rows = get(selection.rows);
      if (rows.length === 0) return;
      const gen = ++seq;
      let choices: BulkChoices;
      try {
        choices = await deps.actions.bulkChangeChoices(rows.map(ref));
      } catch (e) {
        if (gen === seq) failed(e);
        return;
      }
      if (gen !== seq) return;
      opened = { rows, choices, o, input: null };
      const options = valueOptions("status", choices);
      form = {
        stage: "form",
        summary: summaryText(rows.length, choices.kinds),
        field: "status",
        options,
        option: options[0]?.value ?? "",
        values: [],
        text: "",
        error: null,
        busy: false,
      };
      state.set(form);
    },
    setField(field) {
      if (opened === null) return;
      const options = valueOptions(field, opened.choices);
      put({
        field,
        options,
        option: options[0]?.value ?? "",
        values: [],
        text: "",
        error: null,
      });
    },
    setOption(v) {
      put({ option: v, error: null });
    },
    toggleValue(name) {
      if (form === null) return;
      const values = form.values.includes(name)
        ? form.values.filter((v) => v !== name)
        : [...form.values, name];
      put({ values, error: null });
    },
    setText(t) {
      put({ text: t, error: null });
    },
    async submit() {
      const o = opened;
      const f = form;
      if (!o || !f || get(state)?.stage !== "form" || f.busy || !canSubmit(f)) return;
      const gen = ++seq;
      const input = inputFor(f);
      put({ busy: true, error: null });
      let r: BulkChangeResult;
      try {
        r = await deps.actions.bulkChangePreview(o.rows.map(ref), input);
      } catch (e) {
        if (gen !== seq) return;
        if (e instanceof Refused) {
          put({ busy: false, error: e.message });
          return;
        }
        close();
        failed(e);
        await deps.detail.reload();
        await o.o.after();
        return;
      }
      if (gen !== seq) return;
      if (r.changed === 0) {
        close();
        notify(o, r);
        return;
      }
      o.input = input;
      state.set({
        stage: "confirm",
        title: "一括変更",
        lines: confirmLines(input, r),
        ok: `${r.changed} 件を変更`,
      });
    },
    async confirm() {
      const o = opened;
      if (!o || o.input === null || get(state)?.stage !== "confirm" || busy) return;
      busy = true;
      // 実行中に閉じた・開き直したら、新しいダイアログは閉じない(保存は済んでいるので、トーストと読み直しはする)。
      const gen = seq;
      try {
        const r = await deps.actions.bulkChange(o.rows.map(ref), o.input);
        if (gen === seq) close();
        notify(o, r);
      } catch (e) {
        if (gen === seq) close();
        failed(e);
      } finally {
        busy = false;
      }
      await deps.detail.reload();
      await o.o.after();
    },
    cancel() {
      seq += 1;
      close();
    },
  };
}
