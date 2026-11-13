<script lang="ts">
  import { type Resolved, type Route, type StoreNode, syncAllHint, syncCard } from "@hedgehug/core";
  import { Brand, Button, SyncCard } from "@hedgehug/ui";
  import { SvelteSet } from "svelte/reactivity";
  import type { Links } from "../links";
  import { isMac } from "../tab";
  import { itemDialog, sync, syncNow } from "../wire";
  import SidebarStore from "./SidebarStore.svelte";

  // サイドバー(04「サイドバー」)。名前と設定・「新規作成」・Store ごとのブロック・同期のカードを出す。
  interface Props {
    stores: StoreNode[];
    current: Resolved | null;
    links: Links;
  }

  let { stores, current, links }: Props = $props();

  const status = sync.status;
  const card = $derived(syncCard($status, $syncNow));
  const hint = syncAllHint(isMac());
  const settings: Route = { name: "settings", tab: "accounts" };

  // 開閉は、このタブのあいだだけ覚える。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
</script>

<div class="top">
  <Brand />
  <a
    class="gear"
    href={links.href(settings)}
    aria-label="設定"
    title="設定"
    aria-current={current?.name === "settings" ? "page" : undefined}
    onclick={(e) => links.click(e, settings)}
    onauxclick={(e) => links.click(e, settings)}
  >
    <svg width="16" height="16" viewBox="0 0 24 24">
      <title>設定</title>
      <circle cx="12" cy="12" r="3" />
      <path
        d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
      />
    </svg>
  </a>
</div>
<Button variant="primary" onclick={() => void itemDialog.open()}>
  新規作成 <kbd class="kbd">C</kbd>
</Button>
{#each stores as store (store.id)}
  <SidebarStore {store} {current} {links} {collapsed} ontoggle={toggle} />
{/each}
{#if card}
  <SyncCard
    {card}
    {hint}
    onsyncall={() => void sync.syncAll()}
    onsyncstore={(id) => void sync.syncStore(id)}
  />
{/if}

<style>
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .gear {
    display: inline-grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: 8px;
    color: var(--muted);
  }
  .gear:hover,
  .gear[aria-current="page"] {
    background: var(--hover);
    color: var(--ink);
  }
  .gear svg {
    stroke: currentColor;
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .kbd {
    font-family: var(--font-mono);
    font-size: 11px;
    opacity: 0.75;
    margin-left: 4px;
  }
</style>
