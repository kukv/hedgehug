import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type {
  TransferInput,
  TransferPreview,
  TransferResult,
  TransferTarget,
} from "../../model/transfer";
import type { TransfersPort } from "../../port/transfers";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { type Drafts, isItemDraft } from "../drafts/drafts";
import type { Detail } from "../items/detail";
import type { ItemRefresh } from "../items/refresh";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import {
  noTargetsMessage,
  offlineMessage,
  transferFailedMessage,
  transferredMessage,
} from "./dialogText";
import { type Current, formFor, isEpic, keyOf, type TransferDialogState, view } from "./dialogView";

export type { ReplacementRow, TransferDialogState } from "./dialogView";

// 「別のワークスペースへ移動…」のダイアログ(02「移す先の選び方」「保存」、A5a-12〜A5a-14)。
// 候補を読んで先頭のワークスペース・プロジェクト・Epic を選び、移動先が変わるたびに予告を読み直す。
// 「移動」の失敗はトーストを出してダイアログを開いたままにする(NotFound だけは閉じて読み直す)。
// 移したあとの詳細パネルの切り替えと読み直しは afterMoved に任せる。

export interface TransferDialogDeps {
  transfers: Pick<TransfersPort, "targets" | "preview" | "transfer">;
  catalog: Catalog;
  toasts: Toasts;
  refresh: Pick<ItemRefresh, "afterChange">;
  // detail と drafts は、パネルの書きかけを先に聞くためと、送ったときにパネルが item を出していたかを見るために使う。
  detail: Pick<Detail, "view">;
  drafts: Pick<Drafts, "confirmLeave">;
  online(): boolean;
  // afterMoved は、移したあとのパネルの切り替えと読み直し。「移動」を押したときに呼び、成功したら返った関数に結果を
  // 渡す。panelItem は押したときにパネルが出していたアイテムの内部 ID(from かその下のもの。なければ undefined)。
  // 下のものも新しい ID になるので、送っているあいだに読み直しでパネルが閉じても、開き直せるように渡す(02「保存」)。
  afterMoved(from: Item, panelItem: string | undefined): (to: TransferResult) => Promise<void>;
}

export interface TransferDialog {
  state: Readable<TransferDialogState | null>;
  // open は、候補と予告を読んでダイアログを出す。閲覧のみの Store では何もしない。候補がなければトーストだけ。
  open(item: Item): Promise<void>;
  setWorkspace(key: string): Promise<void>;
  setProject(projectId: string): Promise<void>;
  setParent(epicId: string): void;
  setReplacement(status: string, to: string): void;
  submit(): Promise<void>;
  cancel(): void;
}

export function createTransferDialog(deps: TransferDialogDeps): TransferDialog {
  const state = writable<TransferDialogState | null>(null);
  let cur: Current | null = null;
  // seq は、開く・選び直す・閉じるの世代。読んでいるあいだに変わったら、古い結果を捨てる。
  let seq = 0;

  const render = () => state.set(cur ? view(cur) : null);
  const close = () => {
    seq += 1;
    cur = null;
    state.set(null);
  };
  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : transferFailedMessage, { error: true });
  // loadFailed は、候補・予告を読めなかったとき。閉じてトーストを出し、消えていたら画面を読み直す。
  async function loadFailed(e: unknown) {
    close();
    failed(e);
    if (e instanceof NotFound) await deps.refresh.afterChange();
  }

  // load は、いま選んでいる移動先の予告を読む。読んでいる間は loading で、置き換えの選択は初めの値に戻す。
  async function load(c: Current) {
    const gen = ++seq;
    c.preview = null;
    c.choices = {};
    render();
    const [storeId, workspaceId] = [c.target.storeId, c.target.workspaceId];
    let p: TransferPreview;
    try {
      p = await deps.transfers.preview(c.item.storeId, c.item.id, {
        storeId,
        workspaceId,
        projectId: c.form.project,
      });
    } catch (e) {
      if (gen === seq) await loadFailed(e);
      return;
    }
    if (gen !== seq) return;
    c.preview = p;
    c.choices = Object.fromEntries(p.replacements.map((r) => [r.status, r.default]));
    render();
  }

  // shownUnder は、詳細パネルが item か、その下のもの(子・孫)を出していれば、出しているアイテムの内部 ID
  // (内部 ID は Store の中でだけ一意なので Store も比べる)。出していなければ undefined。
  function shownUnder(item: Item): string | undefined {
    const panel = get(deps.detail.view);
    if (panel === null || panel.item.storeId !== item.storeId) return undefined;
    const under = panel.item.id === item.id || panel.ancestors.some((a) => a.id === item.id);
    return under ? panel.item.id : undefined;
  }

  // editable は、選び直せる開いたダイアログ(送っているあいだは選び直さない)。
  const editable = () => (cur && !cur.busy ? cur : null);

  async function send(c: Current) {
    const t = c.target;
    const p = c.preview;
    if (p === null || c.busy || (!isEpic(c.item) && c.form.parent === "")) return;
    // オフラインかは画面が決める。送らずに断るのは、remote のある別の Store へ移すときだけ(同じ Store は送る)。
    if (p.crossStore && p.destinationHasRemote && !deps.online()) {
      deps.toasts.show(offlineMessage(t.storeName), { error: true });
      return;
    }
    const after = deps.afterMoved(c.item, shownUnder(c.item));
    const input: TransferInput = {
      toStore: t.storeId,
      toWorkspace: t.workspaceId,
      toProject: c.form.project,
      ...(isEpic(c.item) ? {} : { toParent: c.form.parent }),
      statusChoices: { ...c.choices },
    };
    c.busy = true;
    render();
    let result: TransferResult;
    try {
      result = await deps.transfers.transfer(c.item.storeId, c.item.id, input);
    } catch (e) {
      c.busy = false;
      failed(e);
      if (e instanceof NotFound) {
        if (cur === c) close();
        await deps.refresh.afterChange();
      } else if (cur === c) render();
      return;
    }
    c.busy = false;
    if (cur === c) close();
    deps.toasts.show(transferredMessage(c.displayId, result.display, result.emptied));
    await after(result);
  }

  return {
    state: readonly(state),
    async open(item) {
      const prefix = deps.catalog.workspace(item.storeId, item.workspaceId)?.prefix;
      if (prefix === undefined || deps.catalog.store(item.storeId)?.readOnly) return;
      const gen = ++seq;
      // 開いているパネルが item か、その祖先のものなら、書きかけの確認(「入力を破棄」)を先に聞く。
      if (shownUnder(item) !== undefined && !(await deps.drafts.confirmLeave(isItemDraft))) return;
      if (gen !== seq) return;
      let targets: TransferTarget[];
      try {
        targets = await deps.transfers.targets(item.storeId, item.id);
      } catch (e) {
        if (gen === seq) await loadFailed(e);
        return;
      }
      if (gen !== seq) return;
      const first = targets[0];
      if (first === undefined) {
        deps.toasts.show(noTargetsMessage);
        return;
      }
      const displayId = itemDisplayId(prefix, item.number);
      const c: Current = {
        item,
        displayId,
        targets,
        target: first,
        form: formFor(item, first),
        preview: null,
        choices: {},
        busy: false,
      };
      cur = c;
      await load(c);
    },
    async setWorkspace(key) {
      const c = editable();
      const t = c?.targets.find((x) => keyOf(x) === key);
      if (!c || !t) return;
      c.target = t;
      c.form = formFor(c.item, t);
      await load(c);
    },
    async setProject(projectId) {
      const c = editable();
      if (!c?.target.projects.some((p) => p.id === projectId)) return;
      c.form = formFor(c.item, c.target, projectId);
      await load(c);
    },
    setParent(epicId) {
      const c = editable();
      if (!c || isEpic(c.item)) return;
      c.form = { ...c.form, parent: epicId };
      render();
    },
    setReplacement(status, to) {
      const c = editable();
      // 予告の行にある元のステータスと、その行の選択肢にある値だけを受ける。
      const row = c?.preview?.replacements.find((r) => r.status === status);
      if (!c || !row?.choices.some((x) => x.id === to)) return;
      c.choices = { ...c.choices, [status]: to };
      render();
    },
    async submit() {
      if (cur) await send(cur);
    },
    cancel: close,
  };
}
