<script lang="ts">
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { workspaceKey } from "../create/context";
  import { readOnlyNow } from "../read-only";

  // Which workspace a sprint or a release goes in, when the screen does not say: the writable ones
  // of the Stores that are shown. The first is taken at once.
  type Props = { entries: WorkspaceEntry[]; onchange: (w: WorkspaceEntry | undefined) => void };
  let { entries, onchange }: Props = $props();

  const c = core();
  const uid = $props.id();
  const choices = $derived(
    entries.filter((w) => !c.stores.hidden(w.store) && readOnlyNow(c.stores, w.store) === null),
  );
  let key = $state("");
  const picked = $derived(choices.find((w) => workspaceKey(w) === key) ?? choices[0]);
  $effect(() => onchange(picked));
</script>

<div class="field">
  <label for={uid}>ワークスペース</label>
  <select
    id={uid}
    value={picked ? workspaceKey(picked) : ""}
    onchange={(e) => (key = e.currentTarget.value)}
  >
    {#each choices as w (workspaceKey(w))}
      <option value={workspaceKey(w)}>{w.name}（{w.prefix}）· {w.store}</option>
    {/each}
  </select>
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  label {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  select {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font-size: var(--fs-14);
  }
</style>
