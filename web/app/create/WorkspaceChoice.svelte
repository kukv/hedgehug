<script lang="ts">
  import type { WorkspaceEntry } from "../../core/stores";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { workspaceKey } from "./context";

  let {
    entries,
    value,
    name,
    onchange,
    label = "ワークスペース",
    showStore = true,
    isDisabled,
    note,
  }: {
    entries: WorkspaceEntry[];
    value: string;
    name: string;
    onchange: (key: string) => void;
    label?: string;
    showStore?: boolean; // the Store badge is left out where every entry is of one Store
    isDisabled?: (w: WorkspaceEntry) => boolean; // an entry that is shown but cannot be chosen
    note?: (w: WorkspaceEntry) => string; // a few words after the name
  } = $props();
  const uid = $props.id();
</script>

<div class="choice" role="radiogroup" aria-label={label}>
  {#each entries as w (workspaceKey(w))}
    <label
      class:on={workspaceKey(w) === value}
      class:off={isDisabled?.(w)}
      title={isDisabled?.(w) ? note?.(w) : undefined}
    >
      <input
        type="radio"
        {name}
        value={workspaceKey(w)}
        checked={workspaceKey(w) === value}
        disabled={isDisabled?.(w)}
        aria-describedby={isDisabled?.(w) && note?.(w) ? `${uid}-${workspaceKey(w)}` : undefined}
        onchange={() => onchange(workspaceKey(w))}
      >
      <span>{w.name}（{w.prefix}）</span>
      {#if note?.(w)}
        <span id="{uid}-{workspaceKey(w)}" class="note">— {note(w)}</span>
      {/if}
      {#if showStore}
        <StoreBadge store={w.store} tone={w.tone} />
      {/if}
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
  .note {
    color: var(--muted);
  }
  label.off {
    color: var(--muted);
    cursor: default;
  }
  label.on {
    border-color: var(--accent);
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
</style>
