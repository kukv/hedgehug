import { type Api, type TaskView, unwrap } from "./api/client";

// taskSearchQuery is the query for the words typed in a search box (the #
// completion, the command palette, the parent and blocker pickers): every
// word is free text that has to be in the ID or the title. Each word is
// quoted, so that a word with a colon or a leading - is not read as a key
// or as a negation. Empty input gives "".
export function taskSearchQuery(text: string, extra = ""): string {
  const words = text
    .split(/\s+/)
    .map((w) => w.replaceAll('"', ""))
    .filter((w) => w !== "");
  return [extra.trim(), ...words.map((w) => `"${w}"`)].filter((t) => t !== "").join(" ");
}

// searchTasks finds the tasks whose ID or title has all the words, in every
// store (or in one). extra is more query ("kind:project", "-status:canceled").
// Nothing is asked for empty input and an empty extra.
export async function searchTasks(
  api: Api,
  text: string,
  opts: { store?: string; extra?: string; limit?: number } = {},
): Promise<TaskView[]> {
  const q = taskSearchQuery(text, opts.extra);
  if (q === "") return [];
  const out = await unwrap(api.GET("/api/tasks", { params: { query: { q, store: opts.store } } }));
  return (out.tasks ?? []).slice(0, opts.limit ?? 8);
}
