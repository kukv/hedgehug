<script lang="ts">
  import { tick } from "svelte";
  import type { WorkspaceEntry } from "../../core/stores";
  import Icon from "../icons/Icon.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";

  // selected holds the ULIDs; none selected is every workspace. onchange gets the next list.
  type Props = {
    workspaces: WorkspaceEntry[];
    selected: string[];
    onchange: (ulids: string[]) => void;
  };
  let { workspaces, selected, onchange }: Props = $props();

  let open = $state(false);
  let word = $state("");
  let root: HTMLElement | undefined = $state();
  let button: HTMLButtonElement | undefined = $state();
  let find: HTMLInputElement | undefined = $state();

  const total = $derived(workspaces.length);
  // The chosen ones that are workspaces still: a ULID of the address that is none is not counted.
  const chosen = $derived(selected.filter((u) => workspaces.some((w) => w.ulid === u)).length);

  // The workspaces whose name or prefix has the word, in the order of the stores.
  const groups = $derived.by(() => {
    const w = word.trim().toLowerCase();
    const hits = workspaces.filter(
      (x) => w === "" || x.name.toLowerCase().includes(w) || x.prefix.toLowerCase().includes(w),
    );
    const out: { store: string; tone: WorkspaceEntry["tone"]; items: WorkspaceEntry[] }[] = [];
    for (const x of hits) {
      const last = out[out.length - 1];
      if (last && last.store === x.store) last.items.push(x);
      else out.push({ store: x.store, tone: x.tone, items: [x] });
    }
    return out;
  });

  async function show() {
    open = true;
    await tick();
    find?.focus();
  }

  function reset() {
    open = false;
    word = "";
  }

  function close() {
    reset();
    button?.focus();
  }

  function toggle(ulid: string) {
    onchange(selected.includes(ulid) ? selected.filter((u) => u !== ulid) : [...selected, ulid]);
  }

  function outside(e: PointerEvent) {
    if (open && root && !root.contains(e.target as Node)) reset();
  }

  // The focus went to something outside (Tab out of the popover): it is not wanted any more.
  function onfocusout(e: FocusEvent) {
    const to = e.relatedTarget as Node | null;
    if (open && to && root && !root.contains(to)) reset();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();
    close();
  }
</script>

<svelte:window onpointerdown={outside} />

<div class="picker" role="presentation" bind:this={root} {onkeydown} {onfocusout}>
  <button
    type="button"
    class="trigger"
    aria-haspopup="dialog"
    aria-expanded={open}
    bind:this={button}
    onclick={() => (open ? close() : void show())}
  >
    <Icon name="workspace" size={13} />
    {chosen === 0 ? total : chosen}
    / {total} ワークスペース
    <Icon name="chevron-down" size={12} />
  </button>
  {#if open}
    <div class="pop" role="dialog" aria-label="ワークスペースで絞り込む">
      <input
        class="find"
        type="text"
        aria-label="ワークスペースを探す"
        placeholder="名前・接頭辞で探す"
        bind:value={word}
        bind:this={find}
      >
      {#each groups as g (g.store)}
        <div class="group">
          <span class="head"><StoreBadge store={g.store} tone={g.tone} compact /></span>
          {#each g.items as w (w.ulid)}
            <label class="row" class:on={selected.includes(w.ulid)}>
              <input
                type="checkbox"
                checked={selected.includes(w.ulid)}
                onchange={() => toggle(w.ulid)}
              >
              <span class="name">{w.name}</span>
              <span class="prefix">{w.prefix}</span>
            </label>
          {/each}
        </div>
      {/each}
      <div class="foot">
        <span class="hint">{chosen} 件を選択中 · 未選択のときはすべて</span>
        <button type="button" class="clear" onclick={() => onchange([])}>選択を解除</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .picker {
    position: relative;
    display: inline-flex;
  }
  .trigger {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    min-height: var(--control-h-sm);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    color: var(--ink-2);
    font: inherit;
    cursor: pointer;
  }
  .pop {
    position: absolute;
    top: calc(100% + var(--space-4));
    left: 0;
    z-index: 20;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    box-sizing: border-box;
    width: 300px;
    max-height: 70vh;
    padding: var(--space-6);
    overflow-y: auto;
    border: 1px solid var(--line);
    border-radius: var(--radius-10);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
  }
  .find {
    box-sizing: border-box;
    min-height: var(--control-h-sm);
    margin: 2px 2px var(--space-4);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font: inherit;
    color: var(--ink);
  }
  .group {
    display: flex;
    flex-direction: column;
  }
  .head {
    padding: var(--space-6) var(--space-8) 2px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border-radius: var(--radius-6);
    cursor: pointer;
  }
  .row.on {
    background: var(--accent-faint);
  }
  .row input {
    margin: 0;
    accent-color: var(--accent);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .prefix {
    padding: 2px var(--space-6);
    border-radius: var(--radius-4);
    background: var(--hover);
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  .foot {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-6) var(--space-4) 2px;
    border-top: 1px solid var(--line);
  }
  .hint {
    flex: 1;
    color: var(--muted);
    font-size: var(--fs-11);
  }
  .clear {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .clear:hover {
    background: var(--hover);
  }
</style>
