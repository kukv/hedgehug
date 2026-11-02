<script lang="ts">
  import type { WorkspaceEntry } from "../../core/stores";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { workspaceKey } from "./context";

  let {
    entries,
    value,
    name,
    onchange,
  }: { entries: WorkspaceEntry[]; value: string; name: string; onchange: (key: string) => void } =
    $props();
</script>

<div class="choice" role="radiogroup" aria-label="ワークスペース">
  {#each entries as w (workspaceKey(w))}
    <label class:on={workspaceKey(w) === value}>
      <input
        type="radio"
        {name}
        value={workspaceKey(w)}
        checked={workspaceKey(w) === value}
        onchange={() => onchange(workspaceKey(w))}
      >
      <span>{w.name}（{w.prefix}）</span>
      <StoreBadge store={w.store} tone={w.tone} />
    </label>
  {/each}
</div>

<style>
  .choice {
    display: grid;
    gap: var(--space-4);
  }
  label {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    padding: var(--space-6) var(--space-10);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    font-size: var(--fs-13);
    cursor: pointer;
  }
  label.on {
    border-color: var(--accent);
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
</style>
