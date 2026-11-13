import { NotFound, Refused } from "../../model/errors";
import type { Member } from "../../model/member";
import type { DescribeResult, NotesPort, NoteTarget } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { type Block, isMemberOf, tokenize } from "../markdown/markdown";
import type { Toasts } from "../toasts/toasts";

// 説明の区画の状態(02「説明」。アイテムとプロジェクトで同じ)。自動保存はない。入力中は、反映で
// 保存してある説明が変わっても入力欄を書き換えない。保存では、編集を始めたときの説明(base)を
// 共通の祖先として送り、行マージはサーバーが行う(A4a-3)。

export const descriptionConflictMessage = "説明の同じ箇所が同時に編集されたため、両方を残しました";

export interface DescriptionState {
  // loaded は、保存してある説明を読み終えたか(読むまで区画の中身を出さない)。
  loaded: boolean;
  // text は、保存してある説明。blocks は、その Markdown(メンションはメンバーのものだけ)。
  text: string;
  blocks: Block[];
  editing: boolean;
  // input は入力欄の中身、base は編集を始めたときの説明。
  input: string;
  base: string;
  saving: boolean;
}

export interface DescriptionDeps {
  port: NotesPort;
  toasts: Toasts;
  // target は、いまの対象。画面への反映で対象の値が新しくなるので、呼ぶたびに引く。
  target(): NoteTarget;
  members(storeId: string): Promise<Member[]>;
}

export interface DescriptionEditor {
  state: Readable<DescriptionState>;
  // load は、保存してある説明を読む(開いたとき・画面への反映のとき)。編集中の入力欄は書き換えない。
  load(): Promise<void>;
  // reset は、対象が別のものに変わったとき、前の説明・入力・読み取りの待ちをすべて捨てて読む前に戻す。
  reset(): void;
  start(): void;
  setInput(text: string): void;
  cancel(): void;
  // save は、説明を保存して入力欄を閉じる。前と同じなら何も保存せずに閉じる。断られたら入力欄を残す。
  save(): Promise<void>;
  // dirty は、入力が編集を始めたときと違うか。held は、違い、かつ空でないか(drafts に渡す)。
  dirty(): boolean;
  held(): boolean;
}

const idle: DescriptionState = {
  loaded: false,
  text: "",
  blocks: [],
  editing: false,
  input: "",
  base: "",
  saving: false,
};

export function createDescriptionEditor(deps: DescriptionDeps): DescriptionEditor {
  const state = writable<DescriptionState>(idle);
  let members: Member[] = [];
  // latest は、読み直しの世代。保存や新しい読み直しのあとに、古い読み取りの結果を捨てる。
  let latest = 0;
  // epoch は、対象が変わった(reset した)回数。待っていた保存が、別の対象のあとに返ったら捨てる。
  let epoch = 0;
  const shown = (text: string) => ({ text, blocks: tokenize(text, isMemberOf(members)) });

  const dirty = () => {
    const s = get(state);
    return s.editing && s.input !== s.base;
  };
  const cancel = () =>
    state.update((s) => ({ ...s, editing: false, input: "", base: "", saving: false }));

  async function load() {
    latest += 1;
    const mine = latest;
    const t = deps.target();
    try {
      const [text, list] = await Promise.all([
        deps.port.description(t),
        deps.members(t.storeId).catch((): Member[] => []),
      ]);
      if (mine !== latest) return;
      members = list;
      state.update((s) => ({ ...s, loaded: true, ...shown(text) }));
    } catch (e) {
      // 開いている対象がなくなったら、パネルは反映で閉じる。ここでは何も出さない。
      if (!(e instanceof NotFound)) throw e;
    }
  }

  async function save() {
    const s = get(state);
    if (!s.editing || s.saving) return;
    // 前と同じ(編集を始めたときのまま、または保存してあるものと同じ)なら、何も保存しない。
    if (s.input === s.base || s.input === s.text) return cancel();
    const mine = epoch;
    state.update((x) => ({ ...x, saving: true }));
    let result: DescribeResult;
    try {
      result = await deps.port.describe(deps.target(), s.base, s.input);
    } catch (e) {
      if (mine !== epoch) return;
      state.update((x) => ({ ...x, saving: false }));
      if (!(e instanceof Refused)) throw e;
      deps.toasts.show(e.message, { error: true });
      return;
    }
    if (mine !== epoch) return;
    latest += 1;
    state.update((x) => ({
      ...x,
      ...shown(result.text),
      editing: false,
      input: "",
      base: "",
      saving: false,
    }));
    if (result.conflict) deps.toasts.show(descriptionConflictMessage);
  }

  return {
    state: readonly(state),
    load,
    reset() {
      epoch += 1;
      latest += 1;
      state.set(idle);
    },
    start() {
      const s = get(state);
      if (!s.loaded || s.editing) return;
      state.update((x) => ({ ...x, editing: true, input: s.text, base: s.text }));
    },
    setInput: (text) => state.update((s) => ({ ...s, input: text })),
    cancel,
    save,
    dirty,
    held: () => dirty() && get(state).input !== "",
  };
}
