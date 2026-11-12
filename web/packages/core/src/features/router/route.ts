// 画面のアドレス(01「画面のアドレス」)。綴りはこの計画の決定 4:
//   /                               ホーム
//   /s/{Store名}/w/{接頭辞}           ワークスペース
//   /s/{Store名}/p/{HH-P1}/{タブ}     プロジェクト
//   /s/{Store名}/i/{HH-12}           アイテムの入口
//   /settings/{タブ}                設定(D2 画面の計画の決定 S3)
// 解釈できないアドレスはホームにする(決定 6)。

export type ProjectTab = "overview" | "tree";

export type SettingsTab = "accounts";

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; prefix: string }
  | { name: "project"; store: string; project: string; tab: ProjectTab }
  | { name: "item"; store: string; item: string }
  | { name: "settings"; tab: SettingsTab };

const projectTabs: readonly string[] = ["overview", "tree"] satisfies ProjectTab[];
const settingsTabs: readonly string[] = ["accounts"] satisfies SettingsTab[];

function decode(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
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
  if (kind === "w" && tab === undefined) return { name: "workspace", store, prefix: id };
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
      return `/s/${e(route.store)}/w/${e(route.prefix)}`;
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
