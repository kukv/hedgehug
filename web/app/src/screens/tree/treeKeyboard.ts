import {
  createTreeWrites,
  get,
  type Item,
  isQueuedAction,
  keepsQueue,
  leftToTarget,
  menuAt,
  projectKeyAction,
  type TreeFocus,
  type TreeKeyAction,
  type TreeRowLike,
  type TreeScreen,
  treeKeyAction,
} from "@hedgehug/core";
import { tick } from "svelte";
import { navigation, toasts } from "../../wire";
import { itemActions, itemKind, itemMenuHost, itemMove } from "../../wireActions";

// ツリーのキー操作(04「ツリーのキー操作」、A4b-20・21、E3a-16)。キーの割り当ては core の treeKeyAction(アイテムの行)と
// projectKeyAction(検索のツリーのプロジェクトの行)、順番待ちと書き込みは core の createTreeWrites。ここは、キーを受けて、
// 画面の部品(詳細パネル・メニュー・ダイアログ)につなぐ。行の型 T は、アイテムか、プロジェクトの行を含む検索のツリーの行。
// 行の id(フォーカス・開閉・data-row)は鍵で、書き込みには itemOf(row).id(内部 ID)を渡す。

export interface TreeKeyboard<T extends TreeRowLike> {
  focus: TreeFocus<T>;
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

export interface TreeKeyboardDeps<T extends TreeRowLike> {
  // focus は、呼ぶ側が作ったフォーカスと開閉(TreeKeyboard の focus はこれを返す)。
  focus: TreeFocus<T>;
  list(): HTMLElement | undefined;
  // itemOf は、行のアイテム。プロジェクトの行は null。
  itemOf(row: T): Item | null;
  // screen は、フォーカスのある行で書く画面(rows はその Store のアイテムの行)。
  screen(): TreeScreen | null;
  readOnly(row: T): boolean;
  // open は Enter、openTab は Ctrl+Enter。
  open(row: T): void;
  openTab(row: T): void;
  // openMenu は、行のメニューを開く(行の「⋯」と同じ)。rect はメニューを出す基準。
  openMenu(row: T, rect: { left: number; bottom: number }): void;
  // create は C(アイテムの行は兄弟、プロジェクトの行はその下の Epic を作る作成ダイアログ)。閲覧のみの行では呼ばない。
  create(row: T): void;
  setStatus(item: Item, statusId: string): Promise<void>;
  // moveProject は、プロジェクトの行の Alt+↑↓(順番待ちに積まない)。先頭・末尾では呼ぶ側が何もしない。閲覧のみの行では呼ばない。
  moveProject?(row: T, dir: "up" | "down"): void;
  afterChange(): Promise<void>;
  // keyOf は、内部 ID から行の鍵を作る(書き込みの側が開く新しい親は内部 ID で来る)。
  keyOf(internalId: string): string;
}

// moves は、行を移るキーの操作。
const moves: ReadonlySet<TreeKeyAction> = new Set(["up", "down", "right", "left"]);

// 入力欄の中、ダイアログを開いているあいだ、メニューを開いているあいだは、ツリーのキーを使わない。
function blocked(event: KeyboardEvent): boolean {
  const target = event.target as HTMLElement | null;
  if (target?.closest("input, textarea, select, [contenteditable='true']")) return true;
  return document.querySelector('[aria-modal="true"], [role="menu"]') !== null;
}

export function createTreeKeyboard<T extends TreeRowLike>(
  deps: TreeKeyboardDeps<T>,
): TreeKeyboard<T> {
  const focus = deps.focus;
  // returning は、メニューを開いたあと、閉じてツリーに戻ったとき、いまの行を先頭に戻さないための印。
  // メニューへ移るときに付け(メニューから別のメニューへ移るときも、いったん戻ったツリーから付け直す)、
  // ツリーに戻ったとき(onfocus)と、メニュー以外へ出たとき(onfocusout)に捨てる。
  let returning = false;
  // pressing は、マウス・タッチで押してから click まで(そのとき `<ul>` に入るフォーカスは、押した行を動かさない)。
  // タッチでは、互換の mousedown とフォーカスが pointerup のあとに来るので、click(または pointercancel)まで残す。
  let pressing = false;
  // refocusing は、行の中のボタンにフォーカスがあるときの移動のキーで、フォーカスを `<ul>` へ移しているあいだ。
  let refocusing = false;
  // data-menu は ui の Menu の浮いた枠(head の検索欄も含む)。data-palette はコマンドパレット(閉じるとフォーカスを
  // 戻すので、メニューと同じく、戻ったときにいまの行を動かさない)。
  const inMenu = (el: EventTarget | null) =>
    el instanceof Element && el.closest("[role='menu'], [data-menu], [data-palette]") !== null;
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

  // actionOf は、行の種類ごとのキーの操作(アイテムの行は treeKeyAction、プロジェクトの行は projectKeyAction)。
  function actionOf(event: KeyboardEvent, row: T): TreeKeyAction | null {
    const item = deps.itemOf(row);
    return item ? treeKeyAction(event, item) : projectKeyAction(event);
  }

  // keyOf は、押したキーのツリーの操作と、フォーカスのある行。ツリーの外・入力欄の中・ダイアログやメニューを
  // 開いているあいだは、操作なし。
  function keyOf(event: KeyboardEvent) {
    const { rows, currentId } = get(focus.state);
    const row = rows.find((r) => r.id === currentId) ?? null;
    const action = row && inTree(event.target) && !blocked(event) ? actionOf(event, row) : null;
    return { action, row };
  }

  // onWindowKey は、どこで押したキーも見る。ほかのキーを押したら(ツリーの外・メニューの中を含む)、待っている分を捨てる。
  function onWindowKey(event: KeyboardEvent) {
    if (!keepsQueue(event, keyOf(event).action)) writes.clear();
  }

  const writes = createTreeWrites({
    screen: () => deps.screen(),
    reorder: (item, rows, dir) => itemMenuHost.reorder(item, rows, dir),
    moveCandidates: (store, id) => itemActions.moveCandidates(store, id),
    moveTo: (item, candidate) => itemMove.moveTo(item, candidate),
    setStatus: (item, status) => deps.setStatus(item, status),
    afterChange: () => deps.afterChange(),
    settled: () => tick(),
    expand: (id) => focus.expand(deps.keyOf(id)),
    toasts,
  });
  // 画面を移ったときは、待っている分を捨てる。
  const stopRoute = navigation.route.subscribe(() => writes.clear());
  window.addEventListener("keydown", onWindowKey, true);

  const popups: ReadonlySet<TreeKeyAction> = new Set(["menu"]);

  // move は、Alt+↑↓。アイテムの行は順番待ちに積み、プロジェクトの行は moveProject を呼ぶ(閲覧のみの行では何もしない)。
  function move(row: T, item: Item | null, dir: "up" | "down") {
    if (item) writes.moveSibling(item.id, dir);
    else if (!deps.readOnly(row)) deps.moveProject?.(row, dir);
  }

  // write は、書き込みのキー。プロジェクトの行で来るのは C と Alt+↑↓ だけ(projectKeyAction)。
  function write(action: TreeKeyAction, row: T) {
    const item = deps.itemOf(row);
    switch (action) {
      case "toggle-done":
        if (item) void writes.toggleDone(item.id);
        break;
      case "status-menu":
        // 行のステータスのボタンを押すのと同じ(閲覧のみでは無効のボタンなので何も起きない)。
        rowEl(row.id)
          ?.querySelector<HTMLButtonElement>("button[aria-label^='ステータス']")
          ?.click();
        break;
      case "change-kind": {
        // 閲覧のみの Store と Epic では、itemKind.open が何もしない。
        if (!item) break;
        const { x, y } = menuAt(rowRect(row.id));
        itemKind.open(item, x, y);
        break;
      }
      case "create-sibling":
        if (!deps.readOnly(row)) deps.create(row);
        break;
      case "move-up":
      case "move-down":
        move(row, item, action === "move-up" ? "up" : "down");
        break;
      case "reparent-up":
      case "reparent-down":
        if (item) writes.changeParent(item.id, action === "reparent-up" ? "up" : "down");
        break;
    }
  }

  function run(action: TreeKeyAction, row: T) {
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
        deps.open(row);
        break;
      case "open-tab":
        deps.openTab(row);
        break;
      case "menu":
        deps.openMenu(row, rowRect(row.id));
        break;
      default:
        write(action, row);
    }
  }

  return {
    focus,
    onkeydown(event) {
      const { action, row } = keyOf(event);
      if (!action || !row) return;
      // 行の中のリンクやボタンにフォーカスがあるときの Enter と、ボタンにフォーカスがあるときの Space は、その部品の
      // 働き(開く・押す)に任せる。
      const inner = event.target !== event.currentTarget;
      const target = !inner
        ? "tree"
        : event.target instanceof HTMLButtonElement
          ? "button"
          : "other";
      if (leftToTarget(action, target)) return;
      // ツリーが受けたキーは、ここで止める(C キーは、App.svelte の「文脈なしで作成ダイアログを開く」へ届けない)。
      event.preventDefault();
      event.stopPropagation();
      // 行の中のボタン(押した開閉・ステータス・「⋯」)にフォーカスがあるまま行を移ると、枠は移った行に、フォーカスは
      // 元の行のボタンに残り、次の Enter が元の行に効く。行を動かすキー(Alt+↑↓・Alt+Shift+↑↓)では、動かした行が
      // 描き直されてボタンが消え、フォーカスがツリーの外に落ちる。どちらも、フォーカスを `<ul>` に移す(いまの行は動かさない)。
      if (inner && (moves.has(action) || isQueuedAction(action))) {
        refocusing = true;
        // スクロールは移動のあとの scrollTo が行う。
        (event.currentTarget as HTMLElement).focus({ preventScroll: true });
        refocusing = false;
      }
      run(action, row);
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
