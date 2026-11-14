import type {
  BacklogFrame,
  PlanningOverview,
  Release,
  ReleaseCard,
  ReleaseInput,
  ReleaseState,
  Sprint,
  SprintCard,
  SprintInput,
} from "../model/planning";

// スプリントとリリースの口。実装は api パッケージにある。
export interface PlanningPort {
  overview(storeId: string, workspaceId: string): Promise<PlanningOverview>;
  sprintCards(storeId: string, workspaceId: string): Promise<SprintCard[]>;
  releaseCards(storeId: string, workspaceId: string): Promise<ReleaseCard[]>;
  backlog(storeId: string, workspaceId: string): Promise<BacklogFrame[]>;
  createSprint(storeId: string, workspaceId: string, input: SprintInput): Promise<Sprint>;
  updateSprint(
    storeId: string,
    workspaceId: string,
    sprintId: string,
    input: SprintInput,
  ): Promise<void>;
  startSprint(
    storeId: string,
    workspaceId: string,
    sprintId: string,
    start: string,
    end: string,
  ): Promise<void>;
  // destination は、未完了のアイテムの移動先(計画中のスプリント)。null はバックログ。
  closeSprint(
    storeId: string,
    workspaceId: string,
    sprintId: string,
    destination: string | null,
  ): Promise<void>;
  deleteSprint(storeId: string, workspaceId: string, sprintId: string): Promise<void>;
  createRelease(storeId: string, workspaceId: string, input: ReleaseInput): Promise<Release>;
  updateRelease(
    storeId: string,
    workspaceId: string,
    releaseId: string,
    input: ReleaseInput,
  ): Promise<void>;
  setReleaseState(
    storeId: string,
    workspaceId: string,
    releaseId: string,
    state: ReleaseState,
  ): Promise<void>;
  deleteRelease(storeId: string, workspaceId: string, releaseId: string): Promise<void>;
  // アイテムの所属は、API がアイテムの宛先で受ける。null はバックログ・「なし」。
  setItemSprint(storeId: string, itemId: string, sprintId: string | null): Promise<void>;
  setItemRelease(storeId: string, itemId: string, releaseId: string | null): Promise<void>;
  // 作業項目を枠の中の位置に置く。sprint は移動先の枠(null はバックログ)、anchor は基準の行
  // (null は枠の末尾)。after が true なら基準の直後、false なら直前。
  placeItem(
    storeId: string,
    itemId: string,
    sprintId: string | null,
    anchorId: string | null,
    after: boolean,
  ): Promise<void>;
}
