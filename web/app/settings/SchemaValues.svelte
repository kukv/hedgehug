<script lang="ts" module>
  /** A value of the list being edited: from is the current values it comes from (none for a new one). */
  export type ValueRow = { key: string; name: string; from: string[] };

  let seq = 0;
  export const newKey = (): string => `v${++seq}`;
</script>

<script lang="ts">
  import { tick } from "svelte";
  import Button from "../ui/Button.svelte";
  import { moved, sortableRow } from "./reorder";
  import SchemaHandle from "./SchemaHandle.svelte";
  import { moveKeys } from "./schema-text";

  // The values of an enum or multi_enum property, of the priorities or of the labels: add, rename (the
  // text), delete, merge into another value, and move. Nothing is written until the list is saved.
  // used says whether a task uses any of the current values a row comes from (such a row cannot be
  // deleted, only merged). keepOne is why the last value cannot be deleted, for a list that needs one.
  type Props = {
    rows: ValueRow[];
    disabled: boolean;
    used: (from: string[]) => boolean;
    keepOne?: string;
  };
  let { rows = $bindable(), disabled, used, keepOne }: Props = $props();

  const uid = $props.id();
  // The row whose merge target is being chosen.
  let merging = $state<string | null>(null);
  let box: HTMLElement | undefined = $state();

  const inputs = (): HTMLInputElement[] => [
    ...(box?.querySelectorAll<HTMLInputElement>("input[type=text]") ?? []),
  ];
  const nameOf = (r: ValueRow): string => r.name.trim();
  // Rows with a name are what a value can be merged into.
  const targets = (except: string): ValueRow[] =>
    rows.filter((o) => o.key !== except && nameOf(o) !== "");

  // After a row is gone the focus goes to the input now at its place, else the one before, else the add button.
  async function focusAfterRemoval(index: number): Promise<void> {
    await tick();
    const list = inputs();
    (list[index] ?? list[index - 1] ?? box?.querySelector<HTMLElement>("[data-add]"))?.focus();
  }

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
    const index = rows.findIndex((r) => r.key === source.key);
    void focusAfterRemoval(index);
    rows = rows
      .filter((r) => r.key !== source.key)
      .map((r) => (r.key === targetKey ? { ...r, from: [...r.from, ...source.from] } : r));
  }
  const remove = (key: string): void => {
    void focusAfterRemoval(rows.findIndex((r) => r.key === key));
    rows = rows.filter((r) => r.key !== key);
  };
  const add = async (): Promise<void> => {
    rows = [...rows, { key: newKey(), name: "", from: [] }];
    await tick();
    inputs().at(-1)?.focus();
  };
  async function startMerge(key: string): Promise<void> {
    merging = key;
    await tick();
    box?.querySelector<HTMLElement>(`select[data-key="${key}"]`)?.focus();
  }
  // Esc closes the choice (and not the dialog); the focus goes back to the button that opened it.
  async function selectKeydown(e: KeyboardEvent, key: string): Promise<void> {
    if (e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();
    merging = null;
    await tick();
    box?.querySelector<HTMLElement>(`button[data-merge="${key}"]`)?.focus();
  }
  const rename = (key: string, name: string): void => {
    rows = rows.map((r) => (r.key === key ? { ...r, name } : r));
  };
  const deleteWhy = "タスクが使っているため削除できません。マージで別の値へ移してください";
  // Why a row cannot be deleted, or undefined when it can.
  const whyNot = (r: ValueRow): string | undefined =>
    keepOne !== undefined && rows.length <= 1 ? keepOne : used(r.from) ? deleteWhy : undefined;
</script>

<div class="values" data-values bind:this={box}>
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
            data-key={r.key}
            {disabled}
            onchange={(e) => merge(r, e.currentTarget.value)}
            onkeydown={(e) => void selectKeydown(e, r.key)}
            onblur={() => (merging = null)}
          >
            <option value="">選んでください</option>
            {#each targets(r.key) as o (o.key)}
              <option value={o.key}>{o.name}</option>
            {/each}
          </select>
        {:else if targets(r.key).length > 0}
          <Button
            size="sm"
            aria-label="マージ（値 {r.name}）"
            data-merge={r.key}
            {disabled}
            onclick={() => void startMerge(r.key)}
          >
            マージ
          </Button>
        {/if}
        <Button
          size="sm"
          aria-label="削除（値 {r.name}）"
          disabled={disabled || whyNot(r) !== undefined}
          why={whyNot(r)}
          onclick={() => remove(r.key)}
        >
          削除
        </Button>
      </li>
    {/each}
  </ul>
  <div>
    <Button size="sm" data-add {disabled} onclick={() => void add()}>値を追加</Button>
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
