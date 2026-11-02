<script lang="ts">
  import { markdown } from "@codemirror/lang-markdown";
  import { Compartment, EditorState, Prec, Transaction } from "@codemirror/state";
  import { keymap, placeholder as placeholderExt } from "@codemirror/view";
  import { EditorView, minimalSetup } from "codemirror";
  import { untrack } from "svelte";
  import { searchTasks } from "../../core/search";
  import { core } from "../context";
  import { mentionExtension } from "./mentions";
  import { peopleOf } from "./people";

  let {
    value,
    store,
    label = "本文",
    placeholder = "Markdown で書けます。@ で人、# でタスク",
    minHeight = "8em",
    pending = false,
    readonly = false,
    onedit,
    oncommit,
    onsubmit,
  }: {
    value: string;
    /** The store whose people and tasks @ and # offer. */
    store: string;
    label?: string;
    placeholder?: string;
    minHeight?: string;
    /** A save of this text is not answered yet: a change from outside waits. */
    pending?: boolean;
    readonly?: boolean;
    onedit: (text: string) => void;
    oncommit?: () => void;
    /** ⌘↵ / Ctrl+↵. Without it the key adds a blank line. */
    onsubmit?: () => void;
  } = $props();

  const c = core();
  let host: HTMLDivElement;
  let view: EditorView | undefined;
  let applying = false;
  const access = new Compartment();

  const theme = EditorView.theme({
    "&": { backgroundColor: "var(--surface)", color: "var(--ink)", fontSize: "var(--fs-13)" },
    ".cm-content": {
      fontFamily: "var(--font-sans)",
      padding: "var(--space-8) 0",
      minHeight: "var(--hh-min-height)",
    },
    "&.cm-focused": { outline: "2px solid var(--accent)" },
    ".cm-cursor": { borderLeftColor: "var(--ink)" },
    ".cm-tooltip.cm-tooltip-autocomplete": {
      border: "1px solid var(--line)",
      borderRadius: "var(--radius-8)",
      backgroundColor: "var(--surface)",
      boxShadow: "var(--shadow-popover)",
    },
    ".cm-tooltip-autocomplete ul li[aria-selected]": {
      backgroundColor: "var(--accent-weak)",
      color: "var(--accent-ink)",
    },
    ".cm-completionDetail": { color: "var(--muted)", marginLeft: "var(--space-8)" },
  });

  function replace(text: string) {
    if (!view) return;
    const doc = view.state.doc.toString();
    if (doc === text) return;
    applying = true;
    view.dispatch({
      changes: { from: 0, to: doc.length, insert: text },
      annotations: Transaction.addToHistory.of(false),
    });
    applying = false;
  }

  // A change from outside is taken only when the editor has no focus and no save is pending.
  function sync() {
    if (!view || view.hasFocus || pending) return;
    replace(value);
  }

  export function getText(): string {
    return view?.state.doc.toString() ?? value;
  }

  /** Replaces the text without reporting it as an edit (the caller sends the change itself). */
  export function setText(text: string) {
    replace(text);
  }

  export function focus() {
    view?.focus();
  }

  $effect(() => {
    const v = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: untrack(() => value),
        extensions: [
          Prec.highest(
            keymap.of([
              {
                key: "Mod-Enter",
                run: () => {
                  if (!onsubmit) return false;
                  onsubmit();
                  return true;
                },
              },
            ]),
          ),
          minimalSetup,
          markdown(),
          EditorView.lineWrapping,
          placeholderExt(untrack(() => placeholder)),
          theme,
          EditorView.contentAttributes.of({ "aria-label": untrack(() => label) }),
          mentionExtension(
            untrack(() => store),
            {
              people: (s) => peopleOf(c).load(s),
              search: (text, s) => searchTasks(c.api, text, { store: s }),
            },
          ),
          access.of([]),
          EditorView.updateListener.of((u) => {
            if (u.docChanged && !applying) onedit(u.state.doc.toString());
          }),
          EditorView.domEventHandlers({
            blur: () => {
              oncommit?.();
              queueMicrotask(sync);
              return false;
            },
          }),
        ],
      }),
    });
    view = v;
    return () => {
      v.destroy();
      view = undefined;
    };
  });

  $effect(() => {
    view?.dispatch({
      effects: access.reconfigure([
        EditorView.editable.of(!readonly),
        EditorState.readOnly.of(readonly),
      ]),
    });
  });

  $effect(() => {
    void value;
    void pending;
    sync();
  });
</script>

<div class="editor" style:--hh-min-height={minHeight} bind:this={host}></div>

<style>
  .editor {
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    padding: 0 var(--space-12);
  }
</style>
