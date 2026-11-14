// 画面のアドレス(01「画面のアドレス」)。綴りはこの計画の決定 4:
//   /                               ホーム
//   /s/{Store名}/w/{接頭辞}/{タブ}     ワークスペース(タブなしは概要)
//   /s/{Store名}/p/{HH-P1}/{タブ}     プロジェクト
//   /s/{Store名}/i/{HH-12}           アイテムの入口
//   /settings/{タブ}                設定(D2 画面の計画の決定 S3)
// 解釈できないアドレスはホームにする(決定 6)。

export type ProjectTab = "overview" | "tree" | "settings";
export type WorkspaceTab = "overview" | "backlog" | "sprints" | "releases" | "reports" | "settings";

export type SettingsTab = "accounts";

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; prefix: string; tab: WorkspaceTab }
  | { name: "project"; store: string; project: string; tab: ProjectTab }
  | { name: "item"; store: string; item: string }
  | { name: "settings"; tab: SettingsTab };

const projectTabs: readonly string[] = ["overview", "tree", "settings"] satisfies ProjectTab[];
// workspaceTabs は、ワークスペースのタブ。画面のタブの並びと同じ順(F2a-13)。
export const workspaceTabs: readonly string[] = [
  "overview",
  "backlog",
  "sprints",
  "releases",
  "reports",
  "settings",
] satisfies WorkspaceTab[];
const settingsTabs: readonly string[] = ["accounts"] satisfies SettingsTab[];

function decode(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
}

// projectTabOf は、タブの ID(Tabs が返す文字列)をプロジェクトのタブにする。知らない ID は概要。
export function projectTabOf(id: string): ProjectTab {
  return projectTabs.includes(id) ? (id as ProjectTab) : "overview";
}

// workspaceTabOf は、タブの ID をワークスペースのタブにする。知らない ID は概要。
export function workspaceTabOf(id: string): WorkspaceTab {
  return workspaceTabs.includes(id) ? (id as WorkspaceTab) : "overview";
}

export function parseRoute(path: string): Route {
  const segs = path
    .split("/")
    .filter((s) => s !== "")
    .map(decode);
  if (segs.some((s) => s === null || s === "")) return { name: "home" };
  if (segs[0] === "settings" && segs.length <= 2) {
    const t = segs[1] ?? "accounts";
    return settingsTabs.includes(t)
      ? { name: "settings", tab: t as SettingsTab }
      : { name: "home" };
  }
  const [s, store, kind, id, tab, ...rest] = segs as string[];
  if (s !== "s" || store === undefined || id === undefined || rest.length > 0) {
    return { name: "home" };
  }
  if (kind === "w") {
    const t = tab ?? "overview";
    if (workspaceTabs.includes(t)) {
      return { name: "workspace", store, prefix: id, tab: t as WorkspaceTab };
    }
  }
  if (kind === "i" && tab === undefined) return { name: "item", store, item: id };
  if (kind === "p") {
    const t = tab ?? "overview";
    if (projectTabs.includes(t)) {
      return { name: "project", store, project: id, tab: t as ProjectTab };
    }
  }
  return { name: "home" };
}

export function formatRoute(route: Route): string {
  const e = encodeURIComponent;
  switch (route.name) {
    case "home":
      return "/";
    case "workspace":
      return `/s/${e(route.store)}/w/${e(route.prefix)}/${route.tab}`;
    case "project":
      return `/s/${e(route.store)}/p/${e(route.project)}/${route.tab}`;
    case "item":
      return `/s/${e(route.store)}/i/${e(route.item)}`;
    case "settings":
      return `/settings/${route.tab}`;
  }
}

// 画面の ID。接頭辞は英大文字で始まる 10 文字以内の英大文字と数字(03)。プロジェクトは番号の前に P。
export type DisplayId = { prefix: string; kind: "project" | "item"; number: number };

const displayIdPattern = /^([A-Z][A-Z0-9]{0,9})-(P?)([1-9][0-9]*)$/;

export function parseDisplayId(s: string): DisplayId | null {
  const m = displayIdPattern.exec(s);
  if (!m) return null;
  const [, prefix = "", p, digits = ""] = m;
  const number = Number(digits);
  if (!Number.isSafeInteger(number)) return null;
  return { prefix, kind: p === "P" ? "project" : "item", number };
}

export function itemDisplayId(prefix: string, number: number): string {
  return `${prefix}-${number}`;
}

export function projectDisplayId(prefix: string, number: number): string {
  return `${prefix}-P${number}`;
}
