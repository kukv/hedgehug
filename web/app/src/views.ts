import {
  createNameDialog,
  createSaveQueryAsView,
  createSearch,
  deleteView,
  duplicateView,
  get,
  menuAt,
  readonly,
  type Search,
  saveAsView,
  type ViewContent,
  viewScreenKey,
  writable,
} from "@hedgehug/core";
import { openSearch, screenStates, search, searchPort, searchSelection } from "./search";
import { navigation } from "./wire";
import { timer, toasts, views, viewsPort } from "./wireBase";

// 保存したビューの組み立て(E3b)。口と一覧は、wire.ts が画面のアドレスの解決に使うので wireBase.ts で作り(wire.ts を
// 読むと循環する)、ここで再公開する。
export { views, viewsPort };

// openView は、そのビューの画面へ移る(履歴に積む)。移れたかを返す(「入力を破棄」の確認で断られたら false)。
export function openView(id: string): Promise<boolean> {
  return navigation.go({ name: "view", id });
}

// createViewSearch は、ビューの画面の検索の流れ(E3b-3)。画面の内容は画面の状態の鍵 view:{id} に置き、鍵に何もない
// ときは一覧のいまの保存した内容(開閉は空)で始める。選択は検索の画面と同じものを使う(E3b-14)。
export function createViewSearch(id: string): Search {
  return createSearch({
    port: searchPort,
    screenStates,
    timer,
    toasts,
    selection: searchSelection,
    screen: viewScreenKey(id),
    fallback: () => get(views.list).find((v) => v.id === id)?.content ?? null,
  });
}

// ダイアログ「ビューとして保存」(E3b-10)。検索の画面のボタンとパレットが openSaveView で開く。
export const saveView = createNameDialog();

export function openSaveView(content: ViewContent): void {
  saveView.open((name) => saveAsView({ views, toasts, open: openView }, name, content));
}

// saveQueryAsView は、パレットの「ビューとして保存…」(E3b-16)。
export const saveQueryAsView = createSaveQueryAsView({
  openSearch,
  settled: () => search.settled(),
  state: () => get(search.state),
  openSave: openSaveView,
});

// ビューの画面の見出しのメニュー(E3b-13)。開いているあいだは、対象のビューと位置を持つ。
export interface ViewTarget {
  id: string;
  name: string;
}

interface OpenViewMenu extends ViewTarget {
  x: number;
  y: number;
}

const menu = writable<OpenViewMenu | null>(null);
export const viewMenuState = readonly(menu);

// openViewMenu は、見出しの「⋯」の下にメニューを開く。rect は「⋯」の getBoundingClientRect()。
export function openViewMenu(view: ViewTarget, rect: { left: number; bottom: number }) {
  menu.set({ ...view, ...menuAt(rect) });
}

export function closeViewMenu() {
  menu.set(null);
}

// 「名前を変更…」のダイアログ。入力欄の初期値は対象のいまの名前。
export const renameView = createNameDialog();
const renameTarget = writable<ViewTarget | null>(null);
export const renameViewTarget = readonly(renameTarget);

export function openRenameView(view: ViewTarget): void {
  renameTarget.set(view);
  renameView.open((name) => views.rename(view.id, name));
}

// 「削除…」の確認。対象があるあいだ確認を出す。
const deleting = writable<ViewTarget | null>(null);
export const deletingView = readonly(deleting);

export function cancelDeleteView(): void {
  deleting.set(null);
}

export async function confirmDeleteView(): Promise<void> {
  const target = get(deleting);
  deleting.set(null);
  if (target) await deleteView({ views, toasts, navigation }, target.id);
}

// selectViewMenu は、メニューの項目を実行して閉じる。
export function selectViewMenu(id: "rename" | "duplicate" | "delete"): void {
  const target = get(menu);
  closeViewMenu();
  if (!target) return;
  const view = { id: target.id, name: target.name };
  if (id === "rename") openRenameView(view);
  else if (id === "delete") deleting.set(view);
  else void duplicateView({ views, open: openView }, view.id);
}

// 画面を移ったら、出しているダイアログを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => {
  saveView.close();
  renameView.close();
  deleting.set(null);
  closeViewMenu();
});
