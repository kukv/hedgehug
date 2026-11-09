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
    onedit,
    oncommit,
  }: {
    store: string;
    value: string[];
    label: string;
    options: string[];
    readonly?: boolean;
    onedit: (v: string[]) => void;
    oncommit: () => void;
  } = $props();

  const rest = $derived(options.filter((o) => !value.includes(o)));
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

<div class="labels" class:readonly>
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
  {#if !readonly && options.length === 0}
    <a {href} onclick={(e) => onLinkClick(e, href)}>設定の「ラベル」で足してください</a>
  {/if}
</div>

<style>
  .labels {
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
