import { Refused } from "../../model/errors";
import type { SettingsPort, SyncTiming } from "../../port/settings";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";

// 設定の「一般」(08「一般」)。ログイン名の欄、同期のタイミングの選択肢、キーボードの一覧。

// failed は、断り(422)でない失敗をエラーのトーストで出す(受信箱・アクティビティと同じ出し方)。
const failed = (toasts: Toasts, e: unknown) =>
  toasts.show(e instanceof Error ? e.message : String(e), { error: true });

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
  // load は、保存してあるログイン名を読む。打っている途中なら欄はそのまま。読めなければエラーのトーストを出す。
  load(): Promise<void>;
  edit(text: string): void;
  // commit は、Enter と欄からフォーカスが外れたときの確定。前後の空白を取り除いて保存してある値と同じなら何もしない。
  // 確定の処理中(確認ダイアログを出しているあいだを含む)は、次の確定を捨てる。
  commit(text: string): Promise<void>;
  // confirm は確認の「変更」、cancel は「キャンセル」(欄を元の値に戻す)。確定の処理中でなければ、どちらも何もしない。
  // cancel は、preview の途中なら、返ってきても確認を開かない。「変更」の保存を待っているあいだでも確認を閉じる。
  confirm(): Promise<void>;
  cancel(): void;
}

// createLoginField は、ログイン名の欄(08 の「ログイン名」と決定 G1-15・G1-16)。検査はサーバーに任せ、断りなら文言を出して元の値に戻す。
// ほかの失敗はエラーのトーストを出し、断りの文言を消し、確認を閉じて元の値に戻す。
export function createLoginField(port: SettingsPort, toasts: Toasts): LoginField {
  let saved = "";
  // busy は、確定の処理中。pending は、確認ダイアログで「変更」を待っている値。running は、口を呼んでいるあいだ。
  // round は、確定とキャンセルのたびに進める。preview が返ったときに違っていれば、その確定はキャンセル済み。
  let busy = false;
  let pending = "";
  let running = false;
  let round = 0;
  const state = writable<LoginFieldState>({ text: "", error: "", confirm: null });
  const patch = (p: Partial<LoginFieldState>) => state.update((s) => ({ ...s, ...p }));

  async function settle(mine: number, fn: () => Promise<void>) {
    running = true;
    try {
      await fn();
    } catch (e) {
      const live = mine === round;
      if (live) pending = "";
      if (e instanceof Refused) {
        if (live) patch({ text: saved, error: e.message, confirm: null });
      } else {
        if (live) patch({ text: saved, error: "", confirm: null });
        failed(toasts, e);
      }
    } finally {
      running = false;
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
      let login: string;
      try {
        login = await port.login();
      } catch (e) {
        return failed(toasts, e);
      }
      if (!busy && get(state).text === saved) patch({ text: login });
      saved = login;
    },
    edit: (text) => patch({ text }),
    async commit(text) {
      const login = text.trim();
      if (busy) return;
      if (login === saved) return patch({ text: saved });
      busy = true;
      round += 1;
      const mine = round;
      await settle(mine, async () => {
        const lines = await port.previewLogin(login);
        if (mine !== round) return;
        if (lines.length === 0) return save(login);
        pending = login;
        patch({ confirm: lines });
      });
    },
    async confirm() {
      if (pending === "") return;
      const login = pending;
      await settle(round, () => save(login));
    },
    cancel() {
      if (!busy) return;
      round += 1;
      pending = "";
      if (!running) busy = false;
      patch({ text: saved, confirm: null });
    },
  };
}

export interface SyncTimingField {
  // state は、出す値。読む前は null。
  state: Readable<SyncTiming | null>;
  // load は、保存済みの値を読む。読めなければエラーのトーストを出す。
  load(): Promise<void>;
  // set は、1 つの項目を変えて保存する(選んだらすぐに有効になる。08)。保存は 1 つずつ順に行い、
  // 前の保存の結果に次の変更を重ねる(後の保存が先の変更を消さない)。失敗したらエラーのトーストを出し、保存済みの値に戻す。
  set(patch: Partial<SyncTiming>): Promise<void>;
}

// createSyncTimingField は、「同期のタイミング」の 4 つの項目。
export function createSyncTimingField(port: SettingsPort, toasts: Toasts): SyncTimingField {
  const state = writable<SyncTiming | null>(null);
  // saved は、保存済みの値。queue は、待っている保存の続き。waiting は、まだ終わっていない保存の数
  // (後ろに保存が待っていれば、先の保存が終わっても、出している後の変更を消さない)。
  let saved: SyncTiming | null = null;
  let queue: Promise<void> = Promise.resolve();
  let waiting = 0;
  const put = (t: SyncTiming) => {
    saved = t;
    state.set(t);
  };

  async function save(patch: Partial<SyncTiming>) {
    try {
      // set は読んだあとだけ保存を並べるので、saved はある(型のための確かめ)。
      if (!saved) return;
      const next = { ...saved, ...patch };
      await port.setTiming(next);
      saved = next;
      if (waiting === 1) state.set(next);
    } catch (e) {
      state.set(saved);
      failed(toasts, e);
    } finally {
      waiting -= 1;
    }
  }

  return {
    state: readonly(state),
    async load() {
      try {
        put(await port.timing());
      } catch (e) {
        failed(toasts, e);
      }
    },
    set(patch) {
      if (!saved) return queue;
      // 選んだ値をすぐに出す(保存を待つあいだ、チェックボックスと選択が元に戻って見えないように)。
      state.update((s) => (s ? { ...s, ...patch } : s));
      waiting += 1;
      queue = queue.then(() => save(patch));
      return queue;
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
