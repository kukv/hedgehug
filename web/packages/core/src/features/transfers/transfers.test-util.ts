import { NotFound, Refused, TransferUndoFailed } from "../../model/errors";
import type {
  Located,
  LocatedProject,
  ProjectTransferPreview,
  ProjectTransferResult,
  ProjectTransferTarget,
  TransferPreview,
  TransferResult,
  TransferTarget,
} from "../../model/transfer";
import type { TransfersPort } from "../../port/transfers";
import { gate } from "../items/fixtures.test-util";

// テスト用の移動の口。呼び出しを calls に残し、答えはテストが決める。
// どの操作も、refuseNext の断り・undoFailNext の取り消しの失敗・failNext の失敗・missing に入れた ID の NotFound を受けられる。
// holdNext は、次の preview・transfer・projectTargets・projectPreview・transferProject を待たせる。

export interface FakeTransfers extends TransfersPort {
  // calls は `targets {id}`・`preview {id} {store}/{workspace}/{project}`・`transfer {id} {JSON}`・`locate {store} {id}`。
  calls: string[];
  targetList: TransferTarget[];
  // previewOf は、移動先(`{store}/{workspace}/{project}`)ごとの予告。ないときは defaultPreview。
  previewOf: Record<string, TransferPreview>;
  defaultPreview: TransferPreview;
  result: TransferResult;
  located: Located;
  // プロジェクトの移動(A5b)。calls は `projectTargets {id}`・`projectPreview {id} {store}/{workspace}`・
  // `transferProject {id} {JSON}`・`locateProject {store} {id}`。projectPreviewOf のキーは `{store}/{workspace}`。
  projectTargetList: ProjectTransferTarget[];
  projectPreviewOf: Record<string, ProjectTransferPreview>;
  defaultProjectPreview: ProjectTransferPreview;
  projectResult: ProjectTransferResult;
  locatedProject: LocatedProject;
  // refuseNext は、次の操作を、この文言の断りにする(1 回だけ)。failNext は、断りでない失敗にする。
  refuseNext(message: string): void;
  failNext(): void;
  // undoFailNext は、次の操作を、この文言の TransferUndoFailed(取り消しもできなかった)にする(1 回だけ)。
  undoFailNext(message: string): void;
  // missing に入れた ID の操作は NotFound になる(locate は引数の ID で見る)。
  missing: Set<string>;
  // holdNext は、release を呼ぶまで、次の preview・transfer・projectTargets・projectPreview・transferProject を待たせる。
  holdNext(): { release: () => void };
}

export function fakeTransfers(): FakeTransfers {
  const calls: string[] = [];
  const missing = new Set<string>();
  let refusal = "";
  let undoFailure = "";
  let failing = false;
  let hold: Promise<void> | null = null;
  // answer は、操作の共通の入口(記録 → 待ち → 失敗 → 断り → ない)。
  const answer = async (id: string, line: string, holdable: boolean) => {
    calls.push(line);
    if (holdable && hold) {
      const h = hold;
      hold = null;
      await h;
    }
    if (failing) {
      failing = false;
      throw new Error("network");
    }
    if (undoFailure !== "") {
      const m = undoFailure;
      undoFailure = "";
      throw new TransferUndoFailed(m);
    }
    if (refusal !== "") {
      const m = refusal;
      refusal = "";
      throw new Refused(m);
    }
    if (missing.has(id)) throw new NotFound("not found");
  };
  const self: FakeTransfers = {
    calls,
    targetList: [],
    previewOf: {},
    defaultPreview: {
      below: 0,
      deleted: 0,
      nextDisplay: "MB-2",
      crossStore: false,
      remoteNotice: false,
      destinationHasRemote: false,
      replacements: [],
    },
    result: { toStore: "S1", itemId: "S1-I9", display: "MB-2", emptied: [] },
    located: { kind: "found", storeId: "S1", projectId: "S1-P1", itemId: "S1-I1" },
    projectTargetList: [],
    projectPreviewOf: {},
    defaultProjectPreview: {
      items: 0,
      deleted: 0,
      prefix: "MB",
      source: "Web",
      destination: "Mobile",
      crossStore: false,
      remoteNotice: false,
      destinationHasRemote: false,
      hasRules: false,
      notCarried: [],
      replacements: [],
    },
    projectResult: { toStore: "S1", projectId: "S1-P1", display: "MB-P2", destination: "Mobile" },
    locatedProject: { kind: "found", storeId: "S1", projectId: "S1-P1" },
    missing,
    refuseNext(message) {
      refusal = message;
    },
    failNext() {
      failing = true;
    },
    undoFailNext(message) {
      undoFailure = message;
    },
    holdNext() {
      const g = gate();
      hold = g.wait;
      return { release: g.release };
    },
    async targets(_s, id) {
      await answer(id, `targets ${id}`, false);
      return self.targetList;
    },
    async preview(_s, id, to) {
      const key = `${to.storeId}/${to.workspaceId}/${to.projectId}`;
      await answer(id, `preview ${id} ${key}`, true);
      return self.previewOf[key] ?? self.defaultPreview;
    },
    async transfer(_s, id, input) {
      await answer(id, `transfer ${id} ${JSON.stringify(input)}`, true);
      return self.result;
    },
    async locate(store, id) {
      await answer(id, `locate ${store} ${id}`, false);
      return self.located;
    },
    async projectTargets(_s, id) {
      const list = self.projectTargetList;
      await answer(id, `projectTargets ${id}`, true);
      return list;
    },
    async projectPreview(_s, id, to) {
      const key = `${to.storeId}/${to.workspaceId}`;
      await answer(id, `projectPreview ${id} ${key}`, true);
      return self.projectPreviewOf[key] ?? self.defaultProjectPreview;
    },
    async transferProject(_s, id, input) {
      await answer(id, `transferProject ${id} ${JSON.stringify(input)}`, true);
      return self.projectResult;
    },
    async locateProject(store, id) {
      await answer(id, `locateProject ${store} ${id}`, false);
      return self.locatedProject;
    },
  };
  return self;
}
