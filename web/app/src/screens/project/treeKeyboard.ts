import {
  createTreeFocus,
  createTreeWrites,
  get,
  type Item,
  keepsQueue,
  menuAt,
  type ProjectView,
  type Route,
  type TreeFocus,
  type TreeKeyAction,
  treeKeyAction,
} from "@hedgehug/core";
import { tick } from "svelte";
import { detail, itemDialog, links, navigation, projectScreen, toasts } from "../../wire";
import { itemActions, itemKind, itemMenuHost, itemMove, itemRefresh } from "../../wireActions";

// ツリーのキー操作(04「ツリーのキー操作」、A4b-20・21)。キーの割り当ては core の treeKeyAction、順番待ちと
// 書き込みは core の createTreeWrites。ここは、キーを受けて、画面の部品(詳細パネル・メニュー・ダイアログ)につなぐ。

export interface TreeKeyboard {
  focus: TreeFocus;
  onkeydown(event: KeyboardEvent): void;
  // onfocus は、ツリー自身にフォーカスが入ったとき(Tab)。先頭の行にフォーカスを付ける(押したとき・メニューから戻ったときを除く)。
  onfocus(event: FocusEvent): void;
  // onfocusout は、ツリーの中からフォーカスが出たとき。メニューへ移るなら、戻ったときにいまの行を動かさない印を付ける。
  onfocusout(event: FocusEvent): void;
  // onpointerdown は、行の中を押したとき。その行にフォーカスを付ける。
  onpointerdown(event: PointerEvent): void;
  // scrollTo は、行が見える位置までスクロールする(キーで行を移ったとき。読み直しでは動かさない)。
  scrollTo(id: string): void;
  // destroy は、ツリーを閉じるとき。待っている分を捨て、画面の移り変わりとキーの見張りをやめる。
  destroy(): void;
}

interface Deps {
  view(): ProjectView;
  list(): HTMLElement | undefined;
  routeOf(item: Item): Route;
  // openMenu は、行のメニューを開く(ProjectTree の openMenu。行の「⋯」と同じ)。rect はメニューを出す基準。
  openMenu(item: Item, rect: { left: number; bottom: number }): void;
}

// moves は、行を移るキーの操作。
const moves: ReadonlySet<TreeKeyAction> = new Set(["up", "down", "right", "left"]);

// 入力欄の中、ダイアログを開いているあいだ、メニューを開いているあいだは、ツリーのキーを使わない。
function blocked(event: KeyboardEvent): boolean {
  const target = event.target as HTMLElement | null;
  if (target?.closest("input, textarea, select, [contenteditable='true']")) return true;
  return document.querySelector('[aria-modal="true"], [role="menu"]') !== null;
}

export function createTreeKeyboard(deps: Deps): TreeKeyboard {
  const focus = createTreeFocus();
  // returning は、メニューを開いたあと、閉じてツリーに戻ったとき、いまの行を先頭に戻さないための印。
  // メニューへ移るときに付け(メニューから別のメニューへ移るときも、いったん戻ったツリーから付け直す)、
  // ツリーに戻ったとき(onfocus)と、メニュー以外へ出たとき(onfocusout)に捨てる。
  let returning = false;
  // pressing は、マウス・タッチで押してから click まで(そのとき `<ul>` に入るフォーカスは、押した行を動かさない)。
  // タッチでは、互換の mousedown とフォーカスが pointerup のあとに来るので、click(または pointercancel)まで残す。
  let pressing = false;
  // refocusing は、行の中のボタンにフォーカスがあるときの移動のキーで、フォーカスを `<ul>` へ移しているあいだ。
  let refocusing = false;
  // data-menu は ui の Menu の浮いた枠(head の検索欄も含む)。
  const inMenu = (el: EventTarget | null) =>
    el instanceof Element && el.closest("[role='menu'], [data-menu]") !== null;
  const inTree = (el: EventTarget | null) =>
    el instanceof Node && deps.list()?.contains(el) === true;

  const rowEl = (id: string) =>
    deps.list()?.querySelector<HTMLElement>(`[data-row="${CSS.escape(id)}"]`) ?? null;
  const rowRect = (id: string) => rowEl(id)?.getBoundingClientRect() ?? { left: 0, bottom: 0 };

  function scrollTo(id: string) {
    rowEl(id)?.scrollIntoView?.({ block: "nearest" });
  }

  function releasePress() {
    pressing = false;
    window.removeEventListener("click", releasePress, true);
    window.removeEventListener("pointercancel", releasePress, true);
  }

  // keyOf は、押したキーのツリーの操作と、フォーカスのある行。ツリーの外・入力欄の中・ダイアログやメニューを
  // 開いているあいだは、操作なし。
  function keyOf(event: KeyboardEvent) {
    const { rows, currentId } = get(focus.state);
    const row = rows.find((r) => r.id === currentId) ?? null;
    const action = inTree(event.target) && !blocked(event) ? treeKeyAction(event, row) : null;
    return { action, row };
  }

  // onWindowKey は、どこで押したキーも見る。ほかのキーを押したら(ツリーの外・メニューの中を含む)、待っている分を捨てる。
  function onWindowKey(event: KeyboardEvent) {
    if (!keepsQueue(event, keyOf(event).action)) writes.clear();
  }

  const writes = createTreeWrites({
    screen: () => {
      const v = deps.view();
      return {
        storeId: v.store.id,
        readOnly: v.store.readOnly,
        settings: v.settings,
        rows: get(focus.state).rows,
      };
    },
    reorder: (item, rows, dir) => itemMenuHost.reorder(item, rows, dir),
    moveCandidates: (store, id) => itemActions.moveCandidates(store, id),
    moveTo: (item, candidate) => itemMove.moveTo(item, candidate),
    setStatus: (id, status) => projectScreen.setStatus(id, status),
    afterChange: () => itemRefresh.afterChange(),
    settled: () => tick(),
    expand: (id) => focus.expand(id),
    toasts,
  });
  // 画面を移ったときは、待っている分を捨てる。
  const stopRoute = navigation.route.subscribe(() => writes.clear());
  window.addEventListener("keydown", onWindowKey, true);

  const popups: ReadonlySet<TreeKeyAction> = new Set(["menu"]);

  function write(action: TreeKeyAction, row: Item, view: ProjectView) {
    switch (action) {
      case "toggle-done":
        void writes.toggleDone(row.id);
        break;
      case "status-menu":
        // 行のステータスのボタンを押すのと同じ(閲覧のみでは無効のボタンなので何も起きない)。
        rowEl(row.id)
          ?.querySelector<HTMLButtonElement>("button[aria-label^='ステータス']")
          ?.click();
        break;
      case "change-kind": {
        // 閲覧のみの Store と Epic では、itemKind.open が何もしない。
        const { x, y } = menuAt(rowRect(row.id));
        itemKind.open(row, x, y);
        break;
      }
      case "create-sibling":
        if (view.store.readOnly) break;
        void itemDialog.open({
          workspaceId: row.workspaceId,
          projectId: row.projectId,
          kind: row.kind,
          parentId: row.parentId,
        });
        break;
      case "move-up":
      case "move-down":
        writes.moveSibling(row.id, action === "move-up" ? "up" : "down");
        break;
      case "reparent-up":
      case "reparent-down":
        writes.changeParent(row.id, action === "reparent-up" ? "up" : "down");
        break;
    }
  }

  function run(action: TreeKeyAction, row: Item, view: ProjectView) {
    if (popups.has(action)) returning = true;
    switch (action) {
      case "up":
      case "down":
      case "right":
      case "left":
        focus.move(action);
        void tick().then(() => {
          const id = get(focus.state).currentId;
          if (id) scrollTo(id);
        });
        break;
      case "open":
        void detail.open(view.store.id, row.id);
        break;
      case "open-tab":
        links.openTab(deps.routeOf(row));
        break;
      case "menu":
        deps.openMenu(row, rowRect(row.id));
        break;
      default:
        write(action, row, view);
    }
  }

  return {
    focus,
    onkeydown(event) {
      const { action, row } = keyOf(event);
      if (!action || !row) return;
      // 行の中のリンクやボタンにフォーカスがあるときの Enter・Space は、その部品の働き(開く・押す)に任せる。
      const inner = event.target !== event.currentTarget;
      if ((action === "open" || action === "toggle-done") && inner) return;
      // ツリーが受けたキーは、ここで止める(C キーは、App.svelte の「文脈なしで作成ダイアログを開く」へ届けない)。
      event.preventDefault();
      event.stopPropagation();
      // 行の中のボタン(押した開閉・ステータス・「⋯」)にフォーカスがあるまま行を移ると、枠は移った行に、フォーカスは
      // 元の行のボタンに残り、次の Enter が元の行に効く。フォーカスを `<ul>` に移す(いまの行は動かさない)。
      if (inner && moves.has(action)) {
        refocusing = true;
        // スクロールは移動のあとの scrollTo が行う。
        (event.currentTarget as HTMLElement).focus({ preventScroll: true });
        refocusing = false;
      }
      run(action, row, deps.view());
    },
    onfocus(event) {
      if (event.target !== event.currentTarget) return;
      // 押して入ったときは、押した行(onpointerdown が付けた)を動かさない。先頭へ動かすと、画面の外の先頭へスクロールして、
      // 押した行ではなく別の行に click が当たる。メニューから戻ったとき(前のフォーカスがメニューか、閉じて消えたとき)も動かさない。
      const back = returning && (event.relatedTarget === null || inMenu(event.relatedTarget));
      focus.enter(back || pressing || refocusing);
      returning = false;
    },
    onfocusout(event) {
      // メニューへ移るときは印を付ける。ツリーの中のメニュー(ステータス)の中で動くときは、印をそのままにする。
      if (inMenu(event.relatedTarget)) returning = true;
      else if (!inMenu(event.target)) returning = false;
    },
    onpointerdown(event) {
      const id = (event.target as HTMLElement).closest<HTMLElement>("[data-row]")?.dataset.row;
      if (id) focus.focus(id);
      // 右・中ボタンでは click も pointercancel も来ず印が残るので、主ボタン(タッチ・ペンも 0)のときだけ付ける。
      if (event.button !== 0) return;
      pressing = true;
      window.addEventListener("click", releasePress, true);
      window.addEventListener("pointercancel", releasePress, true);
    },
    scrollTo,
    destroy() {
      stopRoute();
      writes.clear();
      releasePress();
      window.removeEventListener("keydown", onWindowKey, true);
    },
  };
}
