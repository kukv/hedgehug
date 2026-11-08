<script lang="ts" module>
  /** A value of the list being edited: from is the current values it comes from (none for a new one). */
  export type ValueRow = { key: string; name: string; from: string[] };

  let seq = 0;
  export const newKey = (): string => `v${++seq}`;
</script>

<script lang="ts">
  import Button from "../ui/Button.svelte";
  import { moved, sortableRow } from "./reorder";
  import SchemaHandle from "./SchemaHandle.svelte";
  import { moveKeys } from "./schema-text";

  // The values of an enum or multi_enum property, edited in the dialog: add, rename (the text), delete,
  // merge into another value, and move. Nothing is written until the dialog is saved. used says whether
  // a task uses any of the current values a row comes from (such a row cannot be deleted, only merged).
  type Props = {
    rows: ValueRow[];
    disabled: boolean;
    used: (from: string[]) => boolean;
  };
  let { rows = $bindable(), disabled, used }: Props = $props();

  const uid = $props.id();
  // The row whose merge target is being chosen.
  let merging = $state<string | null>(null);

  const move = (from: number, to: number): void => {
    const keys = moved(
      rows.map((r) => r.key),
      from,
      to,
    );
    rows = keys.flatMap((k) => rows.filter((r) => r.key === k));
  };
  function merge(source: ValueRow, targetKey: string): void {
    merging = null;
    if (targetKey === "") return;
    rows = rows
      .filter((r) => r.key !== source.key)
      .map((r) => (r.key === targetKey ? { ...r, from: [...r.from, ...source.from] } : r));
  }
  const remove = (key: string): void => {
    rows = rows.filter((r) => r.key !== key);
  };
  const add = (): void => {
    rows = [...rows, { key: newKey(), name: "", from: [] }];
  };
  const rename = (key: string, name: string): void => {
    rows = rows.map((r) => (r.key === key ? { ...r, name } : r));
  };
  const deleteWhy = "タスクが使っているため削除できません。マージで別の値へ移してください";
</script>

<div class="values">
  <span id="{uid}-move" class="sr">{moveKeys()} で移動</span>
  <ul>
    {#each rows as r, i (r.key)}
      <li
        use:sortableRow={{
          list: "values",
          name: r.key,
          index: i,
          disabled,
          onmove: move,
        }}
      >
        <SchemaHandle
          id="values/{r.key}"
          subject="値 {r.name}"
          hint="{uid}-move"
          {disabled}
          off={null}
          onstep={(by) => move(i, i + by)}
        />
        <input
          type="text"
          autocomplete="off"
          aria-label="値"
          {disabled}
          value={r.name}
          oninput={(e) => rename(r.key, e.currentTarget.value)}
        >
        {#if merging === r.key}
          <select
            aria-label="マージ先（値 {r.name}）"
            {disabled}
            onchange={(e) => merge(r, e.currentTarget.value)}
          >
            <option value="">選んでください</option>
            {#each rows.filter((o) => o.key !== r.key) as o (o.key)}
              <option value={o.key}>{o.name}</option>
            {/each}
          </select>
        {:else if rows.length > 1}
          <Button
            size="sm"
            aria-label="マージ（値 {r.name}）"
            {disabled}
            onclick={() => (merging = r.key)}
          >
            マージ
          </Button>
        {/if}
        <Button
          size="sm"
          aria-label="削除（値 {r.name}）"
          disabled={disabled || used(r.from)}
          why={used(r.from) ? deleteWhy : undefined}
          onclick={() => remove(r.key)}
        >
          削除
        </Button>
      </li>
    {/each}
  </ul>
  <div>
    <Button size="sm" {disabled} onclick={add}>値を追加</Button>
  </div>
</div>

<style>
  .values {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  ul {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-6);
    align-items: center;
  }
  /* The edge a dragged row would land on (set by sortableRow). */
  .values :global(li[data-edge="top"]) {
    box-shadow: inset 0 2px 0 var(--accent);
  }
  .values :global(li[data-edge="bottom"]) {
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  input[type="text"] {
    flex: 1;
    min-width: 0;
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-14);
  }
  select {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-14);
  }
  input[type="text"]:focus-visible,
  select:focus-visible {
    outline: 2px solid var(--accent);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
