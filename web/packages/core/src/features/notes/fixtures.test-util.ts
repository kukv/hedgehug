import { Refused } from "../../model/errors";
import type { NotesPort, NoteTarget } from "../../port/notes";

export const keyOf = (t: NoteTarget) => `${t.kind}/${t.storeId}/${t.id}`;

export interface FakeNotes extends NotesPort {
  // texts は、対象ごとの保存してある説明(keyOf の形)。
  texts: Map<string, string>;
  // describeCalls は、describe に渡された base と text の記録。
  describeCalls: { base: string; text: string }[];
  // conflictNext は、次の describe を、merged を結果にして conflict: true で返させる(1 回だけ)。
  conflictNext(merged: string): void;
  // refuseNext は、次の describe を、この文言の断りにする(1 回だけ)。
  refuseNext(message: string): void;
}

// テスト用の説明の口。メモリ上に説明を持つ。
export function fakeNotesPort(initial: Record<string, string> = {}): FakeNotes {
  const texts = new Map(Object.entries(initial));
  const describeCalls: { base: string; text: string }[] = [];
  let merged: string | null = null;
  let refusal = "";
  return {
    texts,
    describeCalls,
    conflictNext(text) {
      merged = text;
    },
    refuseNext(message) {
      refusal = message;
    },
    async description(t) {
      return texts.get(keyOf(t)) ?? "";
    },
    async describe(t, base, text) {
      describeCalls.push({ base, text });
      if (refusal !== "") {
        const message = refusal;
        refusal = "";
        throw new Refused(message);
      }
      const saved = merged ?? text;
      const conflict = merged !== null;
      merged = null;
      texts.set(keyOf(t), saved);
      return { text: saved, conflict };
    },
    async listComments() {
      return [];
    },
    async postComment() {
      throw new Error("postComment is not faked");
    },
    async editComment() {
      throw new Error("editComment is not faked");
    },
    async deleteComment() {
      throw new Error("deleteComment is not faked");
    },
  };
}
