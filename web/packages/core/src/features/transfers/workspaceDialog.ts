import { NotFound, Refused, TransferUndoFailed } from "../../model/errors";
import type {
  WorkspaceTransferPreview,
  WorkspaceTransferResult,
  WorkspaceTransferTarget,
} from "../../model/transfer";
import type { TransfersPort } from "../../port/transfers";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { readOnlyNote } from "../catalog/readonly";
import type { Option } from "../projects/createDialog";
import type { Toasts } from "../toasts/toasts";
import { nameMismatch } from "../workspaces/delete";
import { offlineMessage, transferFailedMessage } from "./dialogText";
import {
  noWorkspaceTargetsHiddenMessage,
  noWorkspaceTargetsMessage,
  workspaceMovedMessage,
  workspaceTransferLines,
  workspaceTransferNameLabel,
  workspaceTransferOk,
  workspaceTransferTitle,
} from "./workspaceDialogText";

// ワークスペースの設定「一般」の「別の Store へ移動」の区画と確認ダイアログ(03「別の Store へ移動」、B5c-16・B5c-18)。
// 区画は候補を読んで先頭を選び、「移動…」で選んだ移動先の予告を読んでダイアログを出す(断りならトーストだけ)。
// 名前の照合は画面だけで行う(workspaces/delete.ts と同じ)。送信の失敗はトーストを出してダイアログを開いたままにする
// (NotFound と TransferUndoFailed は閉じて一覧を読み直す)。移したあとの移り先と読み直しは afterMoved に任せる(B5c-17)。

// WorkspaceTransferSubject は、移すワークスペース。
export interface WorkspaceTransferSubject {
  id: string;
  storeId: string;
  name: string;
  prefix: string;
}

export interface WorkspaceTransferSection {
  workspace: WorkspaceTransferSubject;
  // options の値は移動先の Store の ID、見え方は Store 名。
  options: Option[];
  value: string; // 選んでいる移動先(候補がなければ "")
  empty: string; // 候補がなければ「移せる Store がありません」(セレクトの代わりに出す)、あれば ""
  disabled: boolean; // 閲覧のみの Store。セレクトと「移動…」を無効にする
  note: string | undefined; // 閲覧のみの Store では「閲覧のみの Store です」
  canOpen: boolean;
}

export interface WorkspaceTransferDialogState {
  workspace: WorkspaceTransferSubject;
  title: string;
  lines: string[];
  label: string; // 名前の入力欄の名前
  ok: string;
  error: string | null; // 名前が一致しないとき「名前が一致しません」
  busy: boolean;
}

export interface WorkspaceTransferDeps {
  transfers: Pick<TransfersPort, "workspaceTargets" | "workspacePreview" | "transferWorkspace">;
  // catalog は、閲覧のみの Store かを見るためと、なくなっていた・取り消しもできなかったときの読み直しに使う。
  catalog: Pick<Catalog, "store" | "load">;
  toasts: Toasts;
  online(): boolean;
  // afterMoved は、移せたあとの移り先と読み直し。「移動」を押したときに呼び、移せたら返った関数に結果を渡す(B5c-17)。
  afterMoved(ws: WorkspaceTransferSubject): (result: WorkspaceTransferResult) => Promise<void>;
}

export interface WorkspaceTransfer {
  // section は、load で読んだワークスペースの区画(読めていなければ null)。
  section: Readable<WorkspaceTransferSection | null>;
  load(ws: WorkspaceTransferSubject): Promise<void>;
  choose(storeId: string): void;
  // open は、選んでいる移動先の予告を読んで確認ダイアログを出す。閲覧のみの Store・候補なしでは何もない。
  open(): Promise<void>;
  dialog: Readable<WorkspaceTransferDialogState | null>;
  // submit は、入力の前後の空白を除いた値が名前と一致するときだけ送る。一致しないときは error を入れる。
  submit(typed: string): Promise<void>;
  // clearError は、入力が変わったときに、一致しないという表示を消す。
  clearError(): void;
  cancel(): void;
}

interface Section {
  workspace: WorkspaceTransferSubject;
  targets: WorkspaceTransferTarget[];
  hiddenOnly: boolean;
  key: string;
}
interface Current {
  workspace: WorkspaceTransferSubject;
  target: WorkspaceTransferTarget;
  preview: WorkspaceTransferPreview;
  error: string | null;
  busy: boolean;
}

function dialogView(c: Current): WorkspaceTransferDialogState {
  return {
    workspace: c.workspace,
    title: workspaceTransferTitle,
    lines: workspaceTransferLines(c.workspace.name, c.preview),
    label: workspaceTransferNameLabel,
    ok: workspaceTransferOk,
    error: c.error,
    busy: c.busy,
  };
}

export function createWorkspaceTransfer(deps: WorkspaceTransferDeps): WorkspaceTransfer {
  const section = writable<WorkspaceTransferSection | null>(null);
  const dialog = writable<WorkspaceTransferDialogState | null>(null);
  let sec: Section | null = null;
  let cur: Current | null = null;
  // loadSeq は区画を読む世代、seq はダイアログを開く・閉じるの世代。古い結果は捨てる。
  let loadSeq = 0;
  let seq = 0;
  // reading は、予告を読んでいる open の世代(読んでいなければ null)。読んでいるあいだの open は何もしない。
  let reading: number | null = null;

  const readOnly = (s: Section) => deps.catalog.store(s.workspace.storeId)?.readOnly === true;
  const renderSection = () => {
    if (sec === null) return section.set(null);
    const ro = readOnly(sec);
    section.set({
      workspace: sec.workspace,
      options: sec.targets.map((t) => ({ value: t.storeId, label: t.storeName })),
      value: sec.key,
      empty:
        sec.targets.length > 0
          ? ""
          : sec.hiddenOnly
            ? noWorkspaceTargetsHiddenMessage
            : noWorkspaceTargetsMessage,
      disabled: ro,
      note: ro ? readOnlyNote : undefined,
      canOpen: !ro && sec.targets.length > 0,
    });
  };
  const render = () => dialog.set(cur ? dialogView(cur) : null);
  const close = () => {
    seq += 1;
    cur = null;
    dialog.set(null);
  };
  const failed = (e: unknown) =>
    deps.toasts.show(
      e instanceof Refused || e instanceof TransferUndoFailed ? e.message : transferFailedMessage,
      { error: true },
    );
  // reload は、なくなっていた・両方の Store に残ったときの読み直し。失敗は次の読み直しで直るので投げない。
  const reload = () => deps.catalog.load().catch(() => {});

  async function send(c: Current) {
    // オフラインかは画面が決める。送らずに断るのは、移動先に remote があるときだけ(B5c-18)。
    if (c.preview.destinationHasRemote && !deps.online()) {
      deps.toasts.show(offlineMessage(c.target.storeName), { error: true });
      return;
    }
    c.busy = true;
    render();
    const after = deps.afterMoved(c.workspace);
    let result: WorkspaceTransferResult;
    try {
      result = await deps.transfers.transferWorkspace(
        c.workspace.storeId,
        c.workspace.id,
        c.target.storeId,
      );
    } catch (e) {
      c.busy = false;
      failed(e);
      if (e instanceof NotFound || e instanceof TransferUndoFailed) {
        if (cur === c) close();
        await reload();
      } else if (cur === c) render();
      return;
    }
    c.busy = false;
    if (cur === c) close();
    deps.toasts.show(workspaceMovedMessage(result.toStoreName));
    await after(result);
  }

  return {
    section: readonly(section),
    dialog: readonly(dialog),
    async load(ws) {
      const gen = ++loadSeq;
      try {
        const { targets, hiddenOnly } = await deps.transfers.workspaceTargets(ws.storeId, ws.id);
        if (gen !== loadSeq) return;
        // 同じワークスペースを読み直したときは、選んでいた移動先が候補に残っていれば、それを選んだままにする。
        const same = sec?.workspace.storeId === ws.storeId && sec.workspace.id === ws.id;
        const pick = (same ? targets.find((t) => t.storeId === sec?.key) : undefined) ?? targets[0];
        sec = { workspace: ws, targets, hiddenOnly, key: pick?.storeId ?? "" };
      } catch {
        // 読めなかったときは、区画の中身を出さない(次に設定タブを開いたときに読み直す)。
        if (gen !== loadSeq) return;
        sec = null;
      }
      renderSection();
    },
    choose(storeId) {
      if (!sec?.targets.some((t) => t.storeId === storeId)) return;
      sec.key = storeId;
      renderSection();
    },
    async open() {
      const s = sec;
      const target = s?.targets.find((t) => t.storeId === s.key);
      // cancel で世代が進んでいれば、前の読み込みは捨てるので、開き直せる。
      if (!s || !target || readOnly(s) || cur !== null || reading === seq) return;
      const gen = ++seq;
      reading = gen;
      let preview: WorkspaceTransferPreview;
      try {
        preview = await deps.transfers.workspacePreview(
          s.workspace.storeId,
          s.workspace.id,
          target.storeId,
        );
      } catch (e) {
        if (reading === gen) reading = null;
        if (gen !== seq) return;
        failed(e);
        if (e instanceof NotFound) await reload();
        return;
      }
      if (reading === gen) reading = null;
      if (gen !== seq) return;
      cur = { workspace: s.workspace, target, preview, error: null, busy: false };
      render();
    },
    async submit(typed) {
      if (!cur || cur.busy) return;
      if (typed.trim() !== cur.workspace.name) {
        cur.error = nameMismatch;
        render();
        return;
      }
      await send(cur);
    },
    clearError() {
      if (!cur?.error) return;
      cur.error = null;
      render();
    },
    cancel: close,
  };
}
