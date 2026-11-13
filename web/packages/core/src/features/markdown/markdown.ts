import type { Member } from "../../model/member";

// Markdown の字句分け(02「Markdown」「メンション」)。説明とコメントで使える書き方だけを読み、それ以外は
// 文字のまま残す。HTML は作らない(描くのは ui の Markdown.svelte。{@html} は使わない)。

export type Inline =
  | { kind: "text"; text: string }
  | { kind: "bold"; text: string }
  | { kind: "code"; text: string }
  | { kind: "link"; url: string }
  | { kind: "mention"; login: string };

export type Check = "none" | "unchecked" | "checked";

export interface ListItem {
  check: Check;
  inlines: Inline[];
}

export type Block =
  | { kind: "heading"; level: 1 | 2 | 3; inlines: Inline[] }
  | { kind: "list"; ordered: boolean; items: ListItem[] }
  | { kind: "code"; text: string }
  | { kind: "paragraph"; inlines: Inline[] };

// 左から順に、行内のコード・太字・リンク(http と https だけ)・メンション。ログイン名の形は
// internal/domain/member の loginPattern と同じ。
const inlinePattern =
  /`([^`]+)`|\*\*(.+?)\*\*|(https?:\/\/[A-Za-z0-9._~:/?#[\]@!$&'()*+,;=%-]+)|@([A-Za-z0-9_-]{1,39})/g;
const urlTail = /[.,;:!?)'*]+$/;
const urlWithoutHost = /^https?:\/\/$/;
const headingLine = /^(#{1,3})\s+(\S.*)$/;
const listLine = /^\s*(?:[-*]|(\d+\.))\s+(.*)$/;
const checkLine = /^\[( |x)\](?:\s+(.*))?$/;
const fenceLine = /^\s*```/;
// 開きの行は、``` のあとにバッククォートがない行だけ(```foo``` のように 1 行で閉じる行は段落にする)。
const fenceOpen = /^\s*`{3,}[^`]*$/;

// isMemberOf は、メンバーのログイン名と大文字小文字を区別せずに一致するかを返す関数を作る。
export const isMemberOf = (members: Member[]) => {
  const logins = new Set(members.map((m) => m.login.toLowerCase()));
  return (login: string) => logins.has(login.toLowerCase());
};

function inlinesOf(line: string, isMember: (login: string) => boolean): Inline[] {
  const out: Inline[] = [];
  const text = (s: string) => {
    if (s === "") return;
    const last = out.at(-1);
    if (last?.kind === "text") last.text += s;
    else out.push({ kind: "text", text: s });
  };
  let at = 0;
  for (const m of line.matchAll(inlinePattern)) {
    const [whole, code, bold, link, login] = m;
    const index = m.index ?? 0;
    text(line.slice(at, index));
    at = index + (whole?.length ?? 0);
    if (code !== undefined) out.push({ kind: "code", text: code });
    else if (bold !== undefined) out.push({ kind: "bold", text: bold });
    else if (link !== undefined) {
      // 末尾の句読点はリンクに含めない。ホストのない http:// はリンクにしない。
      const url = link.replace(urlTail, "");
      if (urlWithoutHost.test(url)) text(link);
      else {
        out.push({ kind: "link", url });
        at = index + url.length;
      }
    } else if (login !== undefined) {
      // メンバーでない @名前 は、ただの文字(メンションの表示にしない)。
      if (isMember(login)) out.push({ kind: "mention", login });
      else text(whole ?? "");
    }
  }
  text(line.slice(at));
  return out;
}

function listItemOf(rest: string, isMember: (login: string) => boolean): ListItem {
  const box = checkLine.exec(rest);
  if (!box) return { check: "none", inlines: inlinesOf(rest, isMember) };
  return {
    check: box[1] === "x" ? "checked" : "unchecked",
    inlines: inlinesOf(box[2] ?? "", isMember),
  };
}

// tokenize は、本文を行ごとに読んで Block の並びにする。isMember が true の @ログイン名だけをメンションにする。
export function tokenize(text: string, isMember: (login: string) => boolean): Block[] {
  const lines = text.split(/\r?\n/);
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i++] ?? "";
    if (fenceOpen.test(line)) {
      const body: string[] = [];
      while (i < lines.length && !fenceLine.test(lines[i] ?? "")) body.push(lines[i++] ?? "");
      i++; // 閉じの行(なければ最後まで読んでいる)
      blocks.push({ kind: "code", text: body.join("\n") });
      continue;
    }
    if (line.trim() === "") continue;
    const heading = headingLine.exec(line);
    if (heading) {
      const level = (heading[1] ?? "#").length as 1 | 2 | 3;
      const inlines = inlinesOf((heading[2] ?? "").trimEnd(), isMember);
      blocks.push({ kind: "heading", level, inlines });
      continue;
    }
    const row = listLine.exec(line);
    if (row) {
      const ordered = row[1] !== undefined;
      const item = listItemOf((row[2] ?? "").trimEnd(), isMember);
      const last = blocks.at(-1);
      if (last?.kind === "list" && last.ordered === ordered) last.items.push(item);
      else blocks.push({ kind: "list", ordered, items: [item] });
      continue;
    }
    blocks.push({ kind: "paragraph", inlines: inlinesOf(line, isMember) });
  }
  return blocks;
}
