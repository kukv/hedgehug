<script lang="ts">
  import {
    createCollapsed,
    myTasksBadge,
    type Resolved,
    type Route,
    type StoreNode,
    syncAllHint,
    syncCard,
  } from "@hedgehug/core";
  import { Button, SyncCard } from "@hedgehug/ui";
  import { inbox } from "../inbox";
  import type { Links } from "../links";
  import { myTaskBase } from "../myTasks";
  import { openSearchTable, screenStates } from "../search";
  import { isMac } from "../tab";
  import { itemDialog, sync, syncNow } from "../wire";
  import SidebarStore from "./SidebarStore.svelte";
  import SidebarTop from "./SidebarTop.svelte";
  import SidebarViews from "./SidebarViews.svelte";

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
  const myTasksRoute: Route = { name: "mytasks" };
  const inboxRoute: Route = { name: "inbox" };
  const activityRoute: Route = { name: "activity" };
  // 「検索」はテーブルで開く入口(E3a-9)。修飾キーのない主ボタンのクリックで移れたときだけテーブルにする。
  const searchRoute: Route = { name: "search" };
  const unread = inbox.unread;
  // マイタスクのバッジは、絞り込みなしの件数(画面の絞り込みでは変わらない)。
  const myTaskRows = myTaskBase.rows;
  const myTaskCount = $derived($myTaskRows?.length ?? 0);

  // 開閉は画面の状態に残す(G1-14)。has は version を読むので、切り替えと読み込みの完了で描き直される。
  const saved = createCollapsed(screenStates);
  let version = $state(0);
  const collapsed = {
    has: (id: string) => {
      void version;
      return saved.has(id);
    },
  };
  void saved.ready.then(() => version++);
  function toggle(id: string) {
    saved.toggle(
      id,
      stores.flatMap((s) => s.workspaces.map((w) => w.id)),
    );
    version++;
  }
</script>

<SidebarTop {current} {links} />
<Button variant="primary" onclick={() => void itemDialog.open()}>
  新規作成 <kbd class="kbd">C</kbd>
</Button>
<nav class="nav" aria-label="ナビ">
  <a
    class="nav-row"
    href={links.href(myTasksRoute)}
    aria-current={current?.name === "mytasks" ? "page" : undefined}
    onclick={(e) => links.click(e, myTasksRoute)}
    onauxclick={(e) => links.click(e, myTasksRoute)}
  >
    <span>マイタスク</span>
    {#if myTaskCount > 0}
      <span class="badge" role="img" aria-label={myTasksBadge(myTaskCount)}>{myTaskCount}</span>
    {/if}
  </a>
  <a
    class="nav-row"
    href={links.href(inboxRoute)}
    aria-current={current?.name === "inbox" ? "page" : undefined}
    onclick={(e) => links.click(e, inboxRoute)}
    onauxclick={(e) => links.click(e, inboxRoute)}
  >
    <span>受信箱</span>
    {#if $unread.all > 0}
      <span class="badge" role="img" aria-label="未読 {$unread.all} 件">{$unread.all}</span>
    {/if}
  </a>
  <a
    class="nav-row"
    href={links.href(activityRoute)}
    aria-current={current?.name === "activity" ? "page" : undefined}
    onclick={(e) => links.click(e, activityRoute)}
    onauxclick={(e) => links.click(e, activityRoute)}
  >
    <span>アクティビティ</span>
  </a>
  <a
    class="nav-row"
    href={links.href(searchRoute)}
    aria-current={current?.name === "search" ? "page" : undefined}
    onclick={(e) => links.click(e, searchRoute, () => void openSearchTable())}
    onauxclick={(e) => links.click(e, searchRoute)}
  >
    <span>検索</span>
  </a>
</nav>
{#each stores as store (store.id)}
  <SidebarStore {store} {current} {links} {collapsed} ontoggle={toggle} />
{/each}
<SidebarViews {current} {links} />
{#if card}
  <SyncCard
    {card}
    {hint}
    onsyncall={() => void sync.syncAll()}
    onsyncstore={(id) => void sync.syncStore(id)}
  />
{/if}

<style>
  .nav {
    display: flex;
    flex-direction: column;
    padding-top: 8px;
  }
  .nav-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 28px;
    padding: 3px 6px;
    border-radius: 6px;
    color: var(--ink);
    text-decoration: none;
  }
  .nav-row:hover,
  .nav-row[aria-current="page"] {
    background: var(--hover);
  }
  .nav-row[aria-current="page"] {
    font-weight: 500;
  }
  .badge {
    font-size: 11px;
    background: var(--accent);
    color: var(--on-accent);
    border-radius: 999px;
    padding: 0 6px;
    font-variant-numeric: tabular-nums;
  }
  .kbd {
    font-family: var(--font-mono);
    font-size: 11px;
    opacity: 0.75;
    margin-left: 4px;
  }
</style>
