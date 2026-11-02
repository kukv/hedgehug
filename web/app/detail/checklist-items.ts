export type CheckItem = { line: number; checked: boolean; text: string };

// A list item that starts with a checkbox: a bullet or a number, a space, [ ], [x] or [X], then a space or the end.
const checkItem = /^[ \t]*(?:[-*+]|[0-9]{1,9}[.)])[ \t]+\[([ xX])\](?:[ \t]|$)(.*)$/;
// Three or more backticks or tildes after at most three spaces open or close a fenced code block.
const fence = /^ {0,3}(`{3,}|~{3,})/;

export function checklistItems(body: string): CheckItem[] {
  const items: CheckItem[] = [];
  let open = "";
  body.split("\n").forEach((raw, i) => {
    const line = raw.endsWith("\r") ? raw.slice(0, -1) : raw;
    const f = fence.exec(line);
    if (f) {
      const mark = f[1] ?? "";
      if (open === "") {
        open = mark;
      } else if (
        mark.charAt(0) === open.charAt(0) &&
        mark.length >= open.length &&
        line.slice(f[0].length).trim() === ""
      ) {
        open = "";
      }
      return;
    }
    if (open !== "") return;
    const m = checkItem.exec(line);
    if (m) items.push({ line: i, checked: m[1] !== " ", text: (m[2] ?? "").trim() });
  });
  return items;
}

// toggleChecklistLine flips the checkbox on the given line (0-based). A line that is not a checkbox item leaves the body as it is.
export function toggleChecklistLine(body: string, line: number): string {
  if (!checklistItems(body).some((it) => it.line === line)) return body;
  const lines = body.split("\n");
  lines[line] = (lines[line] ?? "").replace(/\[([ xX])\]/, (_m, c: string) =>
    c === " " ? "[x]" : "[ ]",
  );
  return lines.join("\n");
}
