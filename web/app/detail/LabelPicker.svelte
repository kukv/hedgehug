<script lang="ts">
  import { onLinkClick, router } from "../router";

  // The labels of a task: they are chosen from the labels of the schema, never typed. The detail,
  // the cells of the table, the dialog of the kanban, the bulk change and the create dialog share it.
  // A label the list lacks (an old one) is still shown and can be taken off. With an empty list
  // there is nothing to choose: it points to where the labels are added.
  let {
    store,
    value,
    label,
    options,
    readonly = false,
    describedby,
    onnavigate,
    onedit,
    oncommit,
  }: {
    store: string;
    value: string[];
    label: string;
    // undefined: the schema is not read yet; the labels the task holds are shown and nothing is offered.
    options: string[] | undefined;
    readonly?: boolean;
    // The id of the text that says why the labels only read.
    describedby?: string;
    // Called when the link to the settings is followed, for a dialog that has to close to show them.
    onnavigate?: () => void;
    onedit: (v: string[]) => void;
    oncommit: () => void;
  } = $props();

  // A label is told apart from another without regard to case: a task that holds Bug has bug.
  const held = $derived(new Set(value.map((v) => v.toLowerCase())));
  const rest = $derived((options ?? []).filter((o) => !held.has(o.toLowerCase())));
  const href = $derived(router.href({ name: "settings", page: "schema", store, tab: "labels" }));

  function change(next: string[]) {
    onedit(next);
    oncommit();
  }

  function pick(e: Event & { currentTarget: HTMLSelectElement }) {
    const v = e.currentTarget.value;
    e.currentTarget.value = "";
    if (v !== "") change([...value, v]);
  }
</script>

<fieldset class="labels" class:readonly aria-label={label} aria-describedby={describedby}>
  {#each value as v (v)}
    <span class="chip">
      {v}
      {#if !readonly}
        <button
          type="button"
          aria-label="{v} を外す"
          onclick={() => change(value.filter((x) => x !== v))}
        >
          ×
        </button>
      {/if}
    </span>
  {/each}
  {#if !readonly && rest.length > 0}
    <select aria-label={label} value="" onchange={pick}>
      <option value="">追加…</option>
      {#each rest as o (o)}
        <option value={o}>{o}</option>
      {/each}
    </select>
  {/if}
  {#if !readonly && options !== undefined && options.length === 0}
    <a
      {href}
      onclick={(e) => {
        onnavigate?.();
        onLinkClick(e, href);
      }}
      >設定の「ラベル」で足してください</a
    >
  {/if}
</fieldset>

<style>
  .labels {
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-4);
    min-height: var(--control-h-sm);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: 0 var(--space-6);
    border-radius: var(--radius-4);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .chip button {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
  }
  select {
    height: var(--control-h-sm);
    padding: 0 var(--space-6);
    border: 1px solid transparent;
    border-radius: var(--radius-6);
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  select:hover {
    border-color: var(--line-input);
  }
  select:focus-visible {
    outline: 2px solid var(--accent);
    background: var(--surface);
  }
  a {
    color: var(--accent);
    font-size: var(--fs-12);
  }
  .readonly {
    color: var(--ink-2);
  }
</style>
