import { Refused } from "../../model/errors";
import type { SettingsPort, SyncTiming } from "../../port/settings";
import { get, type Readable, readonly, writable } from "../../reactive/store";

// 設定の「一般」(08「一般」)。ログイン名の欄、同期のタイミングの選択肢、キーボードの一覧。

export const intervalChoices: readonly number[] = [1, 5, 15, 30, 60];
export const afterWriteChoices: readonly number[] = [10, 30, 60];
export const githubChoices: readonly number[] = [5, 15, 30, 60];
export const minutesLabel = (n: number) => `${n} 分ごと`;
export const secondsLabel = (n: number) => `${n} 秒`;

export interface LoginFieldState {
  // text は、欄に出す文字。
  text: string;
  // error は、欄の下の断りの文言。
  error: string;
  // confirm は、「ログイン名を変更」の確認の本文の行。開いていなければ null。
  confirm: string[] | null;
}

export interface LoginField {
  state: Readable<LoginFieldState>;
  // load は、保存してあるログイン名を読む。打っている途中なら欄はそのまま。
  load(): Promise<void>;
  edit(text: string): void;
  // commit は、Enter と欄からフォーカスが外れたときの確定。前後の空白を取り除いて保存してある値と同じなら何もしない。
  // 確定の処理中(確認ダイアログを出しているあいだを含む)は、次の確定を捨てる。
  commit(text: string): Promise<void>;
  // confirm は確認の「変更」、cancel は「キャンセル」(欄を元の値に戻す)。確認を出していなければ、どちらも何もしない。
  confirm(): Promise<void>;
  cancel(): void;
}

// createLoginField は、ログイン名の欄(08 の「ログイン名」と決定 G1-15・G1-16)。検査はサーバーに任せ、断りなら文言を出して元の値に戻す。
export function createLoginField(port: SettingsPort): LoginField {
  let saved = "";
  // busy は、確定の処理中。pending は、確認ダイアログで「変更」を待っている値。
  let busy = false;
  let pending = "";
  const state = writable<LoginFieldState>({ text: "", error: "", confirm: null });
  const patch = (p: Partial<LoginFieldState>) => state.update((s) => ({ ...s, ...p }));

  async function settle(fn: () => Promise<void>) {
    try {
      await fn();
    } catch (e) {
      if (!(e instanceof Refused)) throw e;
      patch({ text: saved, error: e.message, confirm: null });
    } finally {
      if (pending === "") busy = false;
    }
  }

  async function save(login: string) {
    pending = "";
    await port.setLogin(login);
    saved = login;
    patch({ text: login, error: "", confirm: null });
  }

  return {
    state: readonly(state),
    async load() {
      const login = await port.login();
      if (!busy && get(state).text === saved) patch({ text: login });
      saved = login;
    },
    edit: (text) => patch({ text }),
    async commit(text) {
      const login = text.trim();
      if (busy) return;
      if (login === saved) return patch({ text: saved });
      busy = true;
      await settle(async () => {
        const lines = await port.previewLogin(login);
        if (lines.length === 0) return save(login);
        pending = login;
        patch({ confirm: lines });
      });
    },
    async confirm() {
      if (pending === "") return;
      const login = pending;
      await settle(() => save(login));
    },
    cancel() {
      if (pending === "") return;
      pending = "";
      busy = false;
      patch({ text: saved, confirm: null });
    },
  };
}

export interface SyncTimingField {
  // state は、読む前は null。
  state: Readable<SyncTiming | null>;
  load(): Promise<void>;
  // set は、1 つの項目を変えて保存する(選んだらすぐに有効になる。08)。
  set(patch: Partial<SyncTiming>): Promise<void>;
}

// createSyncTimingField は、「同期のタイミング」の 4 つの項目。
export function createSyncTimingField(port: SettingsPort): SyncTimingField {
  const state = writable<SyncTiming | null>(null);
  let current: SyncTiming | null = null;
  const put = (t: SyncTiming) => {
    current = t;
    state.set(t);
  };
  return {
    state: readonly(state),
    load: async () => put(await port.timing()),
    async set(patch) {
      if (!current) return;
      const next = { ...current, ...patch };
      await port.setTiming(next);
      put(next);
    },
  };
}

// KeyRow は、「キーボード」の一覧の 1 行(01「キーボード」のキーと動作の文のまま)。
export interface KeyRow {
  key: string;
  action: string;
}

// TreeKeyRow は、「ツリー」の一覧の 1 行(04「ツリーのキー操作」の表のまま)。
export interface TreeKeyRow {
  key: string;
  item: string;
  project: string;
}

// keyboardRows は、01 の表のうち Ctrl+K・C・/・Esc・Shift+Ctrl+S の 5 行をそのままの順で(08)。
export const keyboardRows: readonly KeyRow[] = [
  {
    key: "Ctrl+K(Cmd+K)",
    action:
      "コマンドパレットを開く。ダイアログを開いているときは使えない。パレットから選んだ候補がダイアログを開くときは、パレットを閉じてから開く",
  },
  {
    key: "C",
    action: "アイテムを作成する。入力中とダイアログを開いているときは使えない",
  },
  {
    key: "/",
    action: "絞り込みの入力欄に移る。入力中とダイアログを開いているときは使えない",
  },
  {
    key: "Esc",
    action:
      "開いているものを閉じる。メニュー、絞り込み欄の候補、パレット、ダイアログ、詳細パネル、サイドバーの順。候補が出ているとき、1 回目は候補だけを閉じる。候補は欄にフォーカスがあるあいだだけ出る。入力欄の中身が保存してある内容と違うときは、確認ダイアログ「入力を破棄」を出す(02)。日本語入力の変換中の Esc は、変換の取り消しに使い、この確認の対象にしない",
  },
  {
    key: "Shift+Ctrl+S(Shift+Cmd+S)",
    action: "「すべて同期」を実行する(06)。入力中でも、ダイアログを開いているときでも使える",
  },
];

// treeKeyRows は、04「ツリーのキー操作」の表をそのままの順で(08)。
export const treeKeyRows: readonly TreeKeyRow[] = [
  {
    key: "↑ ↓",
    item: "見えている前・次の行へ動かす。端で止まる(回り込まない)",
    project: "同じ",
  },
  {
    key: "→",
    item: "閉じていて子があれば開く。開いていれば最初の子へ。子がなければ何もしない",
    project: "同じ",
  },
  {
    key: "←",
    item: "開いていれば閉じる。閉じていれば親の行へ。その画面の一番上の階層の行では何もしない",
    project: "開いていれば閉じる。閉じていれば何もしない",
  },
  {
    key: "Enter",
    item: "詳細パネルを開く(タイトルを押したのと同じ)",
    project: "プロジェクトの概要を開く(タイトルを押したのと同じ)",
  },
  {
    key: "Ctrl+Enter(Cmd+Enter)",
    item: "その行の詳細パネルを、新しいブラウザのタブで開く(タイトルを Ctrl+クリックしたのと同じ。01)",
    project: "プロジェクトの概要を、新しいブラウザのタブで開く",
  },
  {
    key: "Space",
    item: "完了と未着手を切り替える。分類が完了でなければ完了の既定のステータスに、完了なら未着手の既定のステータスにする(03)。遷移の制約に反するときは、何も変えず、02「ステータスの変更」の文言をエラーとして表示する",
    project: "何もしない",
  },
  {
    key: "S",
    item: "ステータスのメニューを開く(02)",
    project: "何もしない",
  },
  {
    key: "K",
    item: "「種類を変更…」を開く(02)。Epic は何もしない",
    project: "何もしない",
  },
  {
    key: "Alt+↑、Alt+↓",
    item: "メニューの「上へ移動」「下へ移動」と同じ(絞り込み中は見えている兄弟の間)",
    project: "プロジェクトのメニューの「上へ移動」「下へ移動」と同じ",
  },
  {
    key: "Alt+Shift+↑、Alt+Shift+↓",
    item: "前の親の下へ、次の親の下へ移す(下)",
    project: "何もしない",
  },
  {
    key: "C",
    item: "その行の兄弟として、同じワークスペース・プロジェクト・親・種類で作成ダイアログを開く(02)",
    project: "その行の下に Epic を作る作成ダイアログを開く",
  },
  {
    key: "メニューキー、Shift+F10",
    item: "行のメニューを開く",
    project: "同じ",
  },
];
