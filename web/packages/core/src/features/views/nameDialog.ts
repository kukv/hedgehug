import { type Readable, readonly, writable } from "../../reactive/store";

// ダイアログ「ビューとして保存」の状態(E3b-10)。断りの文(error)だけを持つ。確認の表示は画面が組む。
// 守りは createRenameProjectDialog と同じ。

export interface NameDialogState {
  error: string | null;
}

export interface NameDialog {
  state: Readable<NameDialogState | null>;
  // open の submit は、保存できたら null、断られたらその文を返す。
  open(submit: (name: string) => Promise<string | null>): void;
  // save は、保存できたら閉じ、断られたら開いたまま error に文を入れる。
  save(name: string): Promise<void>;
  close(): void;
}

export function createNameDialog(): NameDialog {
  const state = writable<NameDialogState | null>(null);
  // 開く・閉じるの世代。返事を待つあいだに閉じた・開き直したときは、その返事で状態を変えない。
  let seq = 0;
  let current: ((name: string) => Promise<string | null>) | null = null;
  // 返事を待っている世代。そのあいだの save は何もしない。
  let saving = -1;
  return {
    state: readonly(state),
    open(submit) {
      seq += 1;
      current = submit;
      state.set({ error: null });
    },
    async save(name) {
      const submit = current;
      if (!submit || saving === seq) return;
      const mine = seq;
      saving = mine;
      let message: string | null;
      try {
        message = await submit(name);
      } finally {
        if (saving === mine) saving = -1;
      }
      if (mine !== seq) return;
      state.set(message === null ? null : { error: message });
    },
    close() {
      seq += 1;
      current = null;
      state.set(null);
    },
  };
}
