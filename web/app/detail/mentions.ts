import {
  autocompletion,
  type CompletionContext,
  type CompletionResult,
  type CompletionSource,
} from "@codemirror/autocomplete";
import type { Extension } from "@codemirror/state";
import type { TaskView } from "../../core/api/client";
import { matchPeople } from "../../core/people";

export type MentionDeps = {
  /** The names @ offers in the store. */
  people: (store: string) => Promise<string[]>;
  /** The tasks whose ID or title has the words, in the store. */
  search: (text: string, store: string) => Promise<TaskView[]>;
};

/**
 * triggerOf reads the text before the cursor: an @ or a # that starts a word, and what was typed
 * after it. An @ or # inside a word (an e-mail address, a#b) triggers nothing.
 */
export function triggerOf(before: string): { sigil: "@" | "#"; typed: string } | null {
  const m = /(?:^|[\s(（、。])([@#])([^\s@#]*)$/.exec(before);
  if (!m) return null;
  return { sigil: m[1] as "@" | "#", typed: m[2] ?? "" };
}

/** The completion source for @ (people) and # (tasks) in a Markdown text. */
export function mentionSource(store: string, deps: MentionDeps): CompletionSource {
  return async (ctx: CompletionContext): Promise<CompletionResult | null> => {
    const line = ctx.state.doc.lineAt(ctx.pos);
    const t = triggerOf(line.text.slice(0, ctx.pos - line.from));
    if (!t) return null;
    const from = ctx.pos - t.typed.length;
    if (t.sigil === "@") {
      const names = matchPeople(await deps.people(store), t.typed);
      if (ctx.aborted) return null;
      return {
        from,
        filter: false,
        options: names.map((n) => ({ label: n, apply: `${n} `, type: "text" })),
      };
    }
    if (t.typed === "") return null;
    const found = await deps.search(t.typed, store);
    if (ctx.aborted) return null;
    return {
      from,
      filter: false,
      options: found.map((x) => ({
        label: x.id,
        detail: x.title,
        apply: `${x.id} `,
        type: "text",
      })),
    };
  };
}

/** The autocompletion extension that the body and the comments share. */
export function mentionExtension(store: string, deps: MentionDeps): Extension {
  return autocompletion({
    override: [mentionSource(store, deps)],
    icons: false,
  });
}
