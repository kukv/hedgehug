import { get, type Readable, readonly, writable } from "../../reactive/store";

// スプリント・リリースの作成の要求(A5c-11)。パレットが ask し、そのワークスペースの計画の画面が take で受け取る。

export type PlanKind = "sprint" | "release";
export interface PlanAsk {
  workspaceId: string;
  kind: PlanKind;
}

export interface PlanRequest {
  request: Readable<PlanAsk | null>;
  ask(r: PlanAsk): void;
  // take は、同じワークスペース・種類の要求があれば消して true。ほかは残して false。
  take(workspaceId: string, kind: PlanKind): boolean;
}

export function createPlanRequest(): PlanRequest {
  const state = writable<PlanAsk | null>(null);
  return {
    request: readonly(state),
    ask: (r) => state.set(r),
    take(workspaceId, kind) {
      const current = get(state);
      if (current?.workspaceId !== workspaceId || current.kind !== kind) return false;
      state.set(null);
      return true;
    },
  };
}
