<script lang="ts" module>
  import type { IconName } from "../icons/Icon.svelte";

  /**
   * onselect: the tab does something in place (the layout of a saved view); href is where a new tab opens.
   * disabled: the reason the tab cannot be used; it does nothing when pressed.
   * count: a number shown after the label (left out when it is 0 or not given).
   */
  export type TabItem = {
    label: string;
    href: string;
    current: boolean;
    onselect?: () => void;
    disabled?: string;
    count?: number;
  };
  /** The tab at the right end (the settings), which can have an icon. */
  export type TabEnd = TabItem & { icon?: IconName };
</script>

<script lang="ts">
  import Icon from "../icons/Icon.svelte";
  import { onLinkClick } from "../router";

  type Props = { items: TabItem[]; end?: TabEnd; label?: string };
  let { items, end, label = "ビューの切り替え" }: Props = $props();
  const uid = $props.id();

  function pick(e: MouseEvent, it: TabItem) {
    if (it.disabled) {
      e.preventDefault();
      return;
    }
    const plain = e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
    if (it.onselect && plain && !e.defaultPrevented) {
      e.preventDefault();
      it.onselect();
      return;
    }
    onLinkClick(e, it.href);
  }
</script>

<!-- The tabs are links, so the back button, copying the address and ⌘/Ctrl-click work. -->
<nav class="tabs" aria-label={label}>
  {#each items as it (it.label)}
    <a
      class="tab"
      class:current={it.current}
      href={it.href}
      aria-current={it.current ? "page" : undefined}
      aria-disabled={it.disabled ? "true" : undefined}
      aria-describedby={it.disabled ? `${uid}-${it.label}` : undefined}
      title={it.disabled}
      onclick={(e) => pick(e, it)}
      >{it.label}
      {#if it.count}
        <span class="count">{it.count}</span>
      {/if}</a
    >
    {#if it.disabled}
      <span class="reason" id="{uid}-{it.label}">{it.disabled}</span>
    {/if}
  {/each}
  {#if end}
    <a
      class="tab end"
      class:current={end.current}
      href={end.href}
      aria-current={end.current ? "page" : undefined}
      onclick={(e) => pick(e, end)}
    >
      {#if end.icon}
        <Icon name={end.icon} size={14} />
      {/if}
      {end.label}
    </a>
  {/if}
</nav>

<style>
  .tabs {
    display: flex;
    gap: var(--space-4);
    padding: var(--space-10) var(--gutter) 0;
    border-bottom: var(--hairline) solid var(--line);
  }
  .tab {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    min-height: 2.25rem;
    padding: 0 var(--space-12);
    margin-bottom: calc(var(--hairline) * -1);
    border-bottom: 2px solid transparent;
    color: var(--muted);
    font-size: var(--fs-13);
    text-decoration: none;
  }
  .tab:hover {
    color: var(--ink);
  }
  .count {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    font-weight: 400;
  }
  .reason {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .tab[aria-disabled="true"] {
    color: var(--muted);
    opacity: 0.6;
    cursor: not-allowed;
  }
  .tab.current {
    border-bottom-color: var(--ink);
    color: var(--ink);
    font-weight: 600;
  }
  .end {
    margin-left: auto;
  }
</style>
