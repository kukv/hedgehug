import type { Project } from "../../model/project";
import type { CatalogPort } from "../../port/catalog";

// テスト用の一覧: Store「work」(書ける)と「ro」(閲覧のみ)。どちらもワークスペース Web(HH)と
// プロジェクト HH-P1・HH-P2 を持つ。内部 ID は Store ごとに違う。
export function project(storeId: string, workspaceId: string, n: number): Project {
  return {
    id: `${storeId}-P${n}`,
    storeId,
    workspaceId,
    number: n,
    rank: `V${n}`,
    title: `プロジェクト ${n}`,
    owner: "taro",
    status: "Todo",
    start: "",
    end: "",
    repos: [],
  };
}

export function fakeCatalogPort(): CatalogPort {
  return {
    listStores: async () => [
      { id: "S1", name: "work", color: "teal", readOnly: false },
      { id: "S2", name: "ro", color: "orange", readOnly: true },
    ],
    listWorkspaces: async (storeId) => [
      { id: `${storeId}-W1`, storeId, name: "Web", prefix: "HH" },
    ],
    listProjects: async (storeId, workspaceId) => [
      project(storeId, workspaceId, 1),
      project(storeId, workspaceId, 2),
    ],
    listMembers: async () => [{ login: "taro", name: "太郎" }],
    me: async () => "taro",
  };
}
