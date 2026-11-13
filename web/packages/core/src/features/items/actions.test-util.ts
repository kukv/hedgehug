import { NotFound, Refused } from "../../model/errors";
import type { DeletePreview, MoveCandidate, ProjectDeletePreview } from "../../model/itemAction";
import type { ItemActionsPort } from "../../port/itemActions";
import { gate } from "./fixtures.test-util";

// テスト用のアイテムの操作の口。呼び出しを calls に残し、答えはテストが決める。
// 書く操作(deleteItem・deleteProject・move・changeKind・duplicate)は、refuseNext の断り・
// failNext の失敗・missing に入れた ID の NotFound・holdNext の待ちを受けられる。

export interface FakeItemActions extends ItemActionsPort {
  calls: string[];
  // preview・projectPreview・candidates は、読み取りの答え。
  preview: DeletePreview;
  projectPreview: ProjectDeletePreview;
  candidates: MoveCandidate[];
  // refuseNext は、次の書く操作を、この文言の断りにする(1 回だけ)。failNext は、断りでない失敗にする。
  refuseNext(message: string): void;
  failNext(): void;
  // missing に入れた ID の操作は、NotFound になる(読み取りも)。
  missing: Set<string>;
  // holdNext は、release を呼ぶまで、次の書く操作を待たせる。
  holdNext(): { release: () => void };
}

export function fakeItemActions(): FakeItemActions {
  const calls: string[] = [];
  const missing = new Set<string>();
  let refusal = "";
  let failing = false;
  let hold: Promise<void> | null = null;
  const check = (id: string) => {
    if (missing.has(id)) throw new NotFound("not found");
  };
  // write は、書く操作の共通の入口(待ち → 失敗 → 断り → ない)。
  const write = async (id: string, call: string) => {
    calls.push(call);
    if (hold) {
      const h = hold;
      hold = null;
      await h;
    }
    if (failing) {
      failing = false;
      throw new Error("network");
    }
    if (refusal !== "") {
      const m = refusal;
      refusal = "";
      throw new Refused(m);
    }
    check(id);
  };
  const self: FakeItemActions = {
    calls,
    preview: { count: 0, kinds: [], retentionDays: 30 },
    projectPreview: { items: 0, retentionDays: 30 },
    candidates: [],
    missing,
    refuseNext(message) {
      refusal = message;
    },
    failNext() {
      failing = true;
    },
    holdNext() {
      const g = gate();
      hold = g.wait;
      return { release: g.release };
    },
    async deletePreview(_s, id) {
      calls.push(`deletePreview ${id}`);
      check(id);
      return self.preview;
    },
    deleteItem: (_s, id) => write(id, `deleteItem ${id}`),
    async projectDeletePreview(_s, id) {
      calls.push(`projectDeletePreview ${id}`);
      check(id);
      return self.projectPreview;
    },
    deleteProject: (_s, id) => write(id, `deleteProject ${id}`),
    async moveCandidates(_s, id) {
      calls.push(`moveCandidates ${id}`);
      check(id);
      return self.candidates;
    },
    async movePreview(_s, id, to) {
      calls.push(`movePreview ${id} ${JSON.stringify(to)}`);
      check(id);
      return { below: 0, statuses: [] };
    },
    move: (_s, id, to) => write(id, `move ${id} ${JSON.stringify(to)}`),
    changeKind: (_s, id, kind) => write(id, `changeKind ${id} ${kind}`),
    async duplicate(storeId, id) {
      await write(id, `duplicate ${id}`);
      return `${storeId}-copy-of-${id}`;
    },
  };
  return self;
}
