<script lang="ts">
  import { onDestroy } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { searchTasks } from "../../core/search";
  import { core } from "../context";
  import { useDisplayId } from "../svelte-store";

  let {
    store,
    exclude = [],
    placeholder = "ID かタイトルで検索",
    label = "タスクを検索",
    debounceMs = 150,
    onpick,
  }: {
    store: string;
    /** ULIDs that are not offered (the task itself, what is already added). */
    exclude?: string[];
    placeholder?: string;
    label?: string;
    debounceMs?: number;
    onpick: (t: TaskView) => void;
  } = $props();

  const { api, stores } = core();
  const did = useDisplayId(stores);
  const listId = $props.id();
  let text = $state("");
  let results = $state<TaskView[]>([]);
  let active = $state(0);
  let error = $state("");
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;
  onDestroy(() => clearTimeout(timer));

  function oninput() {
    clearTimeout(timer);
    const q = text.trim();
    if (q === "") {
      seq++;
      results = [];
      return;
    }
    timer = setTimeout(() => void search(q), debounceMs);
  }

  async function search(q: string) {
    const n = ++seq;
    try {
      const found = await searchTasks(api, q, { store, limit: 20 });
      if (n !== seq) return;
      results = found.filter((t) => !exclude.includes(t.ulid)).slice(0, 8);
      active = 0;
      error = "";
    } catch (e) {
      if (n === seq) error = errorMessage(e);
    }
  }

  function pick(t: TaskView) {
    text = "";
    results = [];
    onpick(t);
  }

  function onkeydown(e: KeyboardEvent) {
    // While an IME converts text, Enter and the arrows act on the conversion, not on the list.
    if (e.isComposing || e.keyCode === 229) return;
    if (e.key === "ArrowDown" && results.length > 0) {
      e.preventDefault();
      active = (active + 1) % results.length;
    } else if (e.key === "ArrowUp" && results.length > 0) {
      e.preventDefault();
      active = (active - 1 + results.length) % results.length;
    } else if (e.key === "Enter" && results[active]) {
      e.preventDefault();
      pick(results[active] as TaskView);
    } else if (e.key === "Escape" && results.length > 0) {
      e.stopPropagation();
      results = [];
    }
  }
</script>

<div class="picker">
  <input
    type="text"
    role="combobox"
    aria-label={label}
    aria-expanded={results.length > 0}
    aria-controls={listId}
    autocomplete="off"
    {placeholder}
    bind:value={text}
    {oninput}
    {onkeydown}
  >
  {#if results.length > 0}
    <div id={listId} class="list" role="listbox" aria-label={label}>
      {#each results as t, i (t.ulid)}
        <button
          type="button"
          role="option"
          aria-selected={i === active}
          class:active={i === active}
          onclick={() => pick(t)}
        >
          <span class="id">{did.current(store, t.id)}</span>
          <span class="title">{t.title}</span>
        </button>
      {/each}
    </div>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
</div>

<style>
  .picker {
    position: relative;
  }
  input {
    width: 100%;
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
  }
  .list {
    position: absolute;
    z-index: 5;
    inset-inline: 0;
    margin-top: var(--space-4);
    padding: var(--space-4);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
    display: grid;
  }
  .list button {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    padding: var(--space-6) var(--space-8);
    border: 0;
    border-radius: var(--radius-6);
    background: none;
    font: inherit;
    font-size: var(--fs-13);
    text-align: left;
    color: var(--ink);
    cursor: pointer;
  }
  .list button.active {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .id {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--muted);
  }
  .error {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
