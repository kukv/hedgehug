import { NotFound, Refused, TransferUndoFailed } from "../../model/errors";
import type { Project } from "../../model/project";
import type {
  ProjectTransferPreview,
  ProjectTransferResult,
  ProjectTransferTarget,
} from "../../model/transfer";
import type { TransfersPort } from "../../port/transfers";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { readOnlyNote } from "../catalog/readonly";
import type { Option } from "../projects/createDialog";
import type { Toasts } from "../toasts/toasts";
import { offlineMessage, transferFailedMessage } from "./dialogText";
import type { ReplacementRow } from "./dialogView";
import {
  noProjectTargetsMessage,
  notCarriedLine,
  projectMovedMessage,
  projectTransferIntro,
  projectTransferOk,
  projectTransferTitle,
  replacementLabel,
} from "./projectDialogText";

// プロジェクトの設定の「別のワークスペースへ移動」の区画と確認ダイアログ(02 **移動**、A5b-16・A5b-20)。
// 区画は候補を読んで先頭を選び、「移動」で選んだ移動先の予告を読んでダイアログを出す。
// 送信の失敗はトーストを出してダイアログを開いたままにする(NotFound と TransferUndoFailed は閉じて一覧を読み直す)。
// 移したあとの移り先と読み直しは afterMoved に任せる(A5b-19)。

// ProjectTransferSubject は、移すプロジェクト(ProjectNode も渡せる)。
export type ProjectTransferSubject = Pick<Project, "id" | "storeId" | "title">;

export interface ProjectTransferSection {
  project: ProjectTransferSubject;
  // options の値は `{Store の ID}/{ワークスペースの ID}`、見え方は「{Store 名} / {ワークスペース名}」。
  options: Option[];
  value: string; // 選んでいる移動先(候補がなければ "")
  empty: string; // 候補がなければ「移せるワークスペースがありません」(セレクトの代わりに出す)、あれば ""
  disabled: boolean; // 閲覧のみの Store。セレクトと「移動」を無効にする
  note: string | undefined; // 閲覧のみの Store では「閲覧のみの Store です」
  canOpen: boolean;
}

export interface ProjectTransferDialogState {
  project: ProjectTransferSubject;
  title: string;
  intro: string;
  notCarried: string[]; // 見出し「移らないもの」の下の行(なければ空で、見出しも出さない)
  replacements: ReplacementRow[]; // 見出し「ステータスの置き換え」の下の行(なければ空で、見出しも出さない)
  ok: string;
  busy: boolean;
}

export interface ProjectTransferDeps {
  transfers: Pick<TransfersPort, "projectTargets" | "projectPreview" | "transferProject">;
  // catalog は、閲覧のみの Store かを見るためと、なくなっていた・取り消しもできなかったときの読み直しに使う。
  catalog: Pick<Catalog, "store" | "load">;
  toasts: Toasts;
  online(): boolean;
  // afterMoved は、移せたあと。app が、新しいアドレスの設定タブへ移り、サイドバーの一覧を読み直す(A5b-19)。
  afterMoved(result: ProjectTransferResult, project: ProjectTransferSubject): Promise<void>;
}

export interface ProjectTransfer {
  // section は、load で読んだプロジェクトの区画(読めていなければ null)。
  section: Readable<ProjectTransferSection | null>;
  load(project: ProjectTransferSubject): Promise<void>;
  choose(key: string): void;
  // open は、選んでいる移動先の予告を読んで確認ダイアログを出す。閲覧のみの Store・候補なしでは何もしない。
  open(): Promise<void>;
  dialog: Readable<ProjectTransferDialogState | null>;
  setChoice(status: string, to: string): void;
  submit(): Promise<void>;
  cancel(): void;
}

interface Section {
  project: ProjectTransferSubject;
  targets: ProjectTransferTarget[];
  key: string;
}
interface Current {
  project: ProjectTransferSubject;
  target: ProjectTransferTarget;
  preview: ProjectTransferPreview;
  choices: Record<string, string>; // 元のステータスの内部 ID → 選んだステータスの内部 ID
  busy: boolean;
}

const keyOf = (t: ProjectTransferTarget) => `${t.storeId}/${t.workspaceId}`;

function dialogView(c: Current): ProjectTransferDialogState {
  const p = c.preview;
  return {
    project: c.project,
    title: projectTransferTitle,
    intro: projectTransferIntro(c.project.title, p),
    notCarried: p.notCarried.flatMap((n) => notCarriedLine(n) ?? []),
    replacements: p.replacements.map((r) => ({
      status: r.status,
      name: r.name,
      label: replacementLabel(r.name, r.count),
      options: r.choices.map((x) => ({ value: x.id, label: x.name })),
      value: c.choices[r.status] ?? r.default,
    })),
    ok: projectTransferOk,
    busy: c.busy,
  };
}

export function createProjectTransfer(deps: ProjectTransferDeps): ProjectTransfer {
  const section = writable<ProjectTransferSection | null>(null);
  const dialog = writable<ProjectTransferDialogState | null>(null);
  let sec: Section | null = null;
  let cur: Current | null = null;
  // loadSeq は区画を読む世代、seq はダイアログを開く・閉じるの世代。古い結果は捨てる。
  let loadSeq = 0;
  let seq = 0;

  const readOnly = (s: Section) => deps.catalog.store(s.project.storeId)?.readOnly === true;
  const renderSection = () => {
    if (sec === null) return section.set(null);
    const ro = readOnly(sec);
    section.set({
      project: sec.project,
      options: sec.targets.map((t) => ({
        value: keyOf(t),
        label: `${t.storeName} / ${t.workspaceName}`,
      })),
      value: sec.key,
      empty: sec.targets.length === 0 ? noProjectTargetsMessage : "",
      disabled: ro,
      note: ro ? readOnlyNote : undefined,
      canOpen: !ro && sec.targets.length > 0,
    });
  };
  const render = () => dialog.set(cur ? dialogView(cur) : null);
  const close = () => {
    seq += 1;
    cur = null;
    dialog.set(null);
  };
  const failed = (e: unknown) =>
    deps.toasts.show(
      e instanceof Refused || e instanceof TransferUndoFailed ? e.message : transferFailedMessage,
      { error: true },
    );
  // reload は、なくなっていた・両方の Store に残ったときの読み直し。失敗は次の読み直しで直るので投げない。
  const reload = () => deps.catalog.load().catch(() => {});

  async function send(c: Current) {
    const p = c.preview;
    // オフラインかは画面が決める。送らずに断るのは、remote のある別の Store へ移すときだけ(A5a と同じ)。
    if (p.crossStore && p.destinationHasRemote && !deps.online()) {
      deps.toasts.show(offlineMessage(c.target.storeName), { error: true });
      return;
    }
    c.busy = true;
    render();
    let result: ProjectTransferResult;
    try {
      result = await deps.transfers.transferProject(c.project.storeId, c.project.id, {
        toStore: c.target.storeId,
        toWorkspace: c.target.workspaceId,
        statusChoices: { ...c.choices },
      });
    } catch (e) {
      c.busy = false;
      failed(e);
      if (e instanceof NotFound || e instanceof TransferUndoFailed) {
        if (cur === c) close();
        await reload();
      } else if (cur === c) render();
      return;
    }
    c.busy = false;
    if (cur === c) close();
    deps.toasts.show(projectMovedMessage(result.destination));
    await deps.afterMoved(result, c.project);
  }

  return {
    section: readonly(section),
    dialog: readonly(dialog),
    async load(project) {
      const gen = ++loadSeq;
      try {
        const targets = await deps.transfers.projectTargets(project.storeId, project.id);
        if (gen !== loadSeq) return;
        // 同じプロジェクトを読み直したときは、選んでいた移動先が候補に残っていれば、それを選んだままにする。
        const same = sec?.project.storeId === project.storeId && sec.project.id === project.id;
        const kept = same ? targets.find((t) => keyOf(t) === sec?.key) : undefined;
        const pick = kept ?? targets[0];
        sec = { project, targets, key: pick ? keyOf(pick) : "" };
      } catch {
        // 読めなかったときは、区画の中身を出さない(次に設定タブを開いたときに読み直す)。
        if (gen !== loadSeq) return;
        sec = null;
      }
      renderSection();
    },
    choose(key) {
      if (!sec?.targets.some((t) => keyOf(t) === key)) return;
      sec.key = key;
      renderSection();
    },
    async open() {
      const s = sec;
      const target = s?.targets.find((t) => keyOf(t) === s.key);
      if (!s || !target || readOnly(s) || cur !== null) return;
      const gen = ++seq;
      let preview: ProjectTransferPreview;
      try {
        preview = await deps.transfers.projectPreview(s.project.storeId, s.project.id, {
          storeId: target.storeId,
          workspaceId: target.workspaceId,
        });
      } catch (e) {
        if (gen !== seq) return;
        failed(e);
        if (e instanceof NotFound) await reload();
        return;
      }
      if (gen !== seq) return;
      const choices = Object.fromEntries(preview.replacements.map((r) => [r.status, r.default]));
      cur = { project: s.project, target, preview, choices, busy: false };
      render();
    },
    setChoice(status, to) {
      // 予告の行にある元のステータスと、その行の選択肢にある値だけを受ける。送っているあいだは受けない。
      const row = cur?.preview.replacements.find((r) => r.status === status);
      if (!cur || cur.busy || !row?.choices.some((x) => x.id === to)) return;
      cur.choices = { ...cur.choices, [status]: to };
      render();
    },
    async submit() {
      if (cur && !cur.busy) await send(cur);
    },
    cancel: close,
  };
}
