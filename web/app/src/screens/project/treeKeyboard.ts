import {
  createTreeFocus,
  get,
  type Item,
  type ProjectView,
  type Route,
  type TreeFocus,
  type TreeKeyAction,
  treeKeyAction,
} from "@hedgehug/core";
import { detail, links } from "../../wire";

// ツリーのキー操作(04「ツリーのキー操作」、A4b-20)。キーの割り当ては core の treeKeyAction。
// ここは、キーを受けて、画面の部品(詳細パネル・メニュー)につなぐ。

export interface TreeKeyboard {
  focus: TreeFocus;
  onkeydown(event: KeyboardEvent): void;
  // onfocus は、ツリー自身にフォーカスが入ったとき(Tab)。先頭の行にフォーカスを付ける。
  onfocus(event: FocusEvent): void;
  // onfocusout は、ツリーからフォーカスが出たとき。メニューへ移るのでなければ、ツリーに戻ったとき先頭の行にフォーカスを付ける。
  onfocusout(event: FocusEvent): void;
  // onpointerdown は、行の中を押したとき。その行にフォーカスを付ける。
  onpointerdown(event: PointerEvent): void;
  // scrollTo は、行が見える位置までスクロールする。
  scrollTo(id: string): void;
}

interface Deps {
  view(): ProjectView;
  list(): HTMLElement | undefined;
  routeOf(item: Item): Route;
  // openMenu は、行のメニューを開く(ProjectTree の openMenu。行の「⋯」と同じ)。rect はメニューを出す基準。
  openMenu(item: Item, rect: { left: number; bottom: number }): void;
}

// 入力欄の中、ダイアログを開いているあいだ、メニューを開いているあいだは、ツリーのキーを使わない。
function blocked(event: KeyboardEvent): boolean {
  const target = event.target as HTMLElement | null;
  if (target?.closest("input, textarea, select, [contenteditable='true']")) return true;
  return document.querySelector('[aria-modal="true"], [role="menu"]') !== null;
}

export function createTreeKeyboard(deps: Deps): TreeKeyboard {
  const focus = createTreeFocus();
  // returning は、キーでメニューを開いたあと、閉じてツリーに戻ったとき、いまの行を先頭に戻さないための印。
  // メニューのあいだ(メニューから別のメニューへ移るときも)は残し、ほかへ出たときに捨てる(onfocusout)。
  let returning = false;
  // pressing は、マウス・タッチで押した直後(そのとき `<ul>` に入るフォーカスは、押した行を動かさない)。
  let pressing = false;

  const rowEl = (id: string) =>
    deps.list()?.querySelector<HTMLElement>(`[data-row="${CSS.escape(id)}"]`) ?? null;
  const rowRect = (id: string) => rowEl(id)?.getBoundingClientRect() ?? { left: 0, bottom: 0 };

  const popups: ReadonlySet<TreeKeyAction> = new Set(["menu"]);

  function run(action: TreeKeyAction, row: Item, view: ProjectView) {
    if (popups.has(action)) returning = true;
    switch (action) {
      case "up":
      case "down":
      case "right":
      case "left":
        focus.move(action);
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
    }
  }

  return {
    focus,
    onkeydown(event) {
      if (blocked(event)) return;
      const { rows, currentId } = get(focus.state);
      const row = rows.find((r) => r.id === currentId) ?? null;
      const action = treeKeyAction(event, row);
      if (!action || !row) return;
      // 行の中のリンクやボタンにフォーカスがあるときの Enter は、その部品の働き(開く・押す)に任せる。
      if (action === "open" && event.target !== event.currentTarget) return;
      // ツリーが受けたキーは、ここで止める。
      event.preventDefault();
      event.stopPropagation();
      run(action, row, deps.view());
    },
    onfocus(event) {
      if (event.target !== event.currentTarget) return;
      // 押して入ったときは、押した行(onpointerdown が付けた)を動かさない。先頭へ動かすと、画面の外の先頭へスクロールして、
      // 押した行ではなく別の行に click が当たる。
      focus.enter(returning || pressing);
    },
    onfocusout(event) {
      // メニューの中から出るとき(閉じて部品が消えるときを含む)は、戻ってくるので捨てない。
      const from = event.target as HTMLElement;
      const to = event.relatedTarget as HTMLElement | null;
      if (!from.closest("[role='menu']") && !to?.closest("[role='menu']")) returning = false;
    },
    onpointerdown(event) {
      const id = (event.target as HTMLElement).closest<HTMLElement>("[data-row]")?.dataset.row;
      if (id) focus.focus(id);
      // フォーカスの移動は、押した直後の同じ作業の中で起きる。次の作業で印を外す。
      pressing = true;
      setTimeout(() => {
        pressing = false;
      }, 0);
    },
    scrollTo(id) {
      rowEl(id)?.scrollIntoView?.({ block: "nearest" });
    },
  };
}
