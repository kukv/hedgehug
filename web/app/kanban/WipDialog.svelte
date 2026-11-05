<script lang="ts">
  import { untrack } from "svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import type { Column } from "./board";

  type Props = {
    columns: Column[];
    wip: Record<string, number> | undefined;
    onsave: (wip: Record<string, number> | undefined) => void;
    onclose: () => void;
  };
  let { columns, wip, onsave, onclose }: Props = $props();

  let values = $state<Record<string, string>>(
    untrack(() => Object.fromEntries(Object.entries(wip ?? {}).map(([k, v]) => [k, String(v)]))),
  );

  // A column without a number has no limit.
  function save() {
    const out = Object.entries(values).flatMap(([k, v]) => {
      const n = Number(v);
      return Number.isInteger(n) && n >= 1 ? [[k, n] as const] : [];
    });
    onsave(out.length > 0 ? Object.fromEntries(out) : undefined);
    onclose();
  }
</script>

<Dialog open title="WIP 上限" width={560} {onclose}>
  <p class="note">上限を超えても移動はできます。列の見出しが警告の色になります。</p>
  <div class="rows">
    {#each columns as col (col.key)}
      <label>
        <span>{col.label}</span>
        <input
          type="number"
          min="1"
          aria-label="{col.label} の上限"
          value={values[col.key] ?? ""}
          oninput={(e) => (values[col.key] = e.currentTarget.value)}
        >
      </label>
    {/each}
  </div>
  {#snippet footer()}
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" onclick={save}>適用</Button>
  {/snippet}
</Dialog>

<style>
  .note {
    margin: 0 0 var(--space-12);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .rows {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  label {
    display: flex;
    gap: var(--space-12);
    align-items: center;
    justify-content: space-between;
  }
  input {
    width: 6em;
    min-height: 32px;
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
  }
</style>
