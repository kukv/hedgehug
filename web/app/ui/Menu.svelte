<script lang="ts" module>
  export type MenuItem = { value: string; label: string; disabled?: boolean; hint?: string };
</script>

<script lang="ts">
  import { onMount, type Snippet, tick } from "svelte";

  type Props = {
    anchor: HTMLElement | null;
    items: MenuItem[];
    label?: string;
    onselect: (value: string) => void;
    onclose: () => void;
    selected?: string;
    item?: Snippet<[MenuItem]>;
  };
  let { anchor, items, label = "メニュー", onselect, onclose, selected, item }: Props = $props();

  let root: HTMLElement | undefined = $state();
  let active = $state(0);
  let pos = $state({ top: 0, left: 0 });

  function enabled(i: number): boolean {
    return !items[i]?.disabled;
  }

  function firstEnabled(): number {
    const i = items.findIndex((it) => !it.disabled);
    return Math.max(i, 0);
  }

  function lastEnabled(): number {
    for (let i = items.length - 1; i >= 0; i--) if (enabled(i)) return i;
    return 0;
  }

  function step(from: number, dir: 1 | -1): number {
    for (let n = 1; n <= items.length; n++) {
      const i = (from + dir * n + items.length * n) % items.length;
      if (enabled(i)) return i;
    }
    return from;
  }

  // Below the anchor, or above it when there is no room; centered when there is no anchor.
  function place() {
    if (!root) return;
    const h = root.offsetHeight;
    const w = root.offsetWidth;
    if (!anchor) {
      pos = {
        top: Math.max(0, (window.innerHeight - h) / 3),
        left: Math.max(0, (window.innerWidth - w) / 2),
      };
      return;
    }
    const r = anchor.getBoundingClientRect();
    let top = r.bottom + 4;
    if (top + h > window.innerHeight && r.top - 4 - h >= 0) top = r.top - 4 - h;
    pos = { top, left: Math.max(0, Math.min(r.left, window.innerWidth - w)) };
  }

  async function focusActive() {
    await tick();
    root?.querySelectorAll<HTMLElement>('[role="menuitem"]')[active]?.focus();
  }

  function move(i: number) {
    active = i;
    focusActive();
  }

  onMount(() => {
    place();
    const at = items.findIndex((it) => it.value === selected && !it.disabled);
    active = at >= 0 ? at : firstEnabled();
    focusActive();
  });

  function choose(i: number) {
    const it = items[i];
    if (!it || it.disabled) return;
    onselect(it.value);
  }

  function onkeydown(e: KeyboardEvent) {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        move(step(active, 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        move(step(active, -1));
        break;
      case "Home":
        e.preventDefault();
        move(firstEnabled());
        break;
      case "End":
        e.preventDefault();
        move(lastEnabled());
        break;
      case "Enter":
      case " ":
        e.preventDefault();
        choose(active);
        break;
      case "Escape":
        e.preventDefault();
        e.stopPropagation();
        onclose();
        break;
      case "Tab":
        onclose();
        break;
    }
  }

  function onwindowpointerdown(e: PointerEvent) {
    const t = e.target as Node | null;
    if (!root || root.contains(t) || anchor?.contains(t)) return;
    onclose();
  }
</script>

<svelte:window onpointerdown={onwindowpointerdown} />

<div
  class="menu"
  role="menu"
  aria-label={label}
  tabindex="-1"
  bind:this={root}
  style:top="{pos.top}px"
  style:left="{pos.left}px"
  {onkeydown}
>
  {#each items as it, i (it.value)}
    <button
      type="button"
      role="menuitem"
      class="item"
      class:selected={it.value === selected}
      tabindex={i === active ? 0 : -1}
      disabled={it.disabled}
      onclick={() => choose(i)}
      onmouseenter={() => {
        if (!it.disabled) active = i;
      }}
    >
      {#if item}
        {@render item(it)}
      {:else}
        <span class="label">{it.label}</span>
      {/if}
      {#if it.hint}
        <span class="hint">{it.hint}</span>
      {/if}
    </button>
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 120;
    display: flex;
    flex-direction: column;
    min-width: 180px;
    max-height: calc(100vh - 2 * var(--space-16));
    padding: var(--space-4);
    overflow-y: auto;
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
  }
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    text-align: left;
    cursor: pointer;
  }
  .item:hover:not(:disabled),
  .item:focus-visible {
    background: var(--hover);
    outline: none;
  }
  .item.selected {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .item:disabled {
    color: var(--muted-2);
    cursor: not-allowed;
  }
  .label {
    flex: 1;
  }
  .hint {
    color: var(--label);
    font-size: var(--fs-11);
  }
</style>
