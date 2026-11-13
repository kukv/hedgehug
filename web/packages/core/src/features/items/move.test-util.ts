import type { MoveCandidate } from "../../model/itemAction";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeItemActions } from "./actions.test-util";
import { item } from "./fixtures.test-util";
import { createItemMove } from "./move";
import { fakeWorkspaces, settings } from "./settings.test-util";

// 移動のテストの共通部分。「プロジェクト 1」(S1-P1)の Task HH-2 を動かす。候補は、同じプロジェクトの Epic HH-1、別のプロジェクトの Epic HH-9。
export const epic: MoveCandidate = {
  kind: "item",
  id: "S1-I1",
  display: "HH-1",
  title: "認証",
  projectId: "S1-P1",
  projectTitle: "プロジェクト 1",
};
export const far: MoveCandidate = {
  kind: "item",
  id: "S1-I9",
  display: "HH-9",
  title: "決済",
  projectId: "S1-P2",
  projectTitle: "プロジェクト 2",
};
export const project: MoveCandidate = {
  kind: "project",
  id: "S1-P2",
  display: "HH-P2",
  title: "プロジェクト 2",
  projectId: "S1-P2",
  projectTitle: "",
};

export async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeItemActions();
  actions.candidates = [epic, far];
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const refreshed: string[] = [];
  const move = createItemMove({
    actions,
    catalog,
    toasts,
    // 「プロジェクト 2」(S1-P2)は、Pending(st5)を使わない。
    workspaces: fakeWorkspaces({
      "S1-W1": {
        ...settings,
        statuses: [
          ...settings.statuses,
          { id: "st5", name: "Pending", color: "gray", category: "todo" },
        ],
        projectRules: {
          "S1-P2": {
            hiddenStatuses: ["st5"],
            workflow: {},
            templates: [],
            templateDefaults: {},
            disabledRules: [],
          },
        },
      },
    }),
    refresh: {
      afterChange: async () => {
        refreshed.push("change");
      },
    },
  });
  const task = item("S1", "S1-P1", 2, "Task", "S1-I1", "画面");
  const ro = item("S2", "S2-P1", 1, "Task", null, "閲覧のみ");
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { move, actions, refreshed, messages, task, ro };
}
