<script lang="ts">
  import { emptyOf, type InboxTab, inboxTabs, type Member } from "@hedgehug/core";
  import { Button, PageHead, Tabs } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { SvelteMap } from "svelte/reactivity";
  import { inbox } from "../../inbox";
  import { listMembers } from "../../wire";
  import InboxRow from "./InboxRow.svelte";

  // 受信箱の画面(06「受信箱」)。タブごとに未読数を出し、0 件なら数字を出さない。
  interface Props {
    ondrawer: () => void;
    alert: boolean;
  }

  let { ondrawer, alert }: Props = $props();

  const view = inbox.state;
  const unread = inbox.unread;
  const canReadAll = inbox.canReadAll;
  const tabs = $derived(
    inboxTabs.map((t) => ({ ...t, ...($unread[t.id] > 0 ? { count: $unread[t.id] } : {}) })),
  );
  const empty = $derived(emptyOf($view.tab));

  onMount(() => void inbox.open());

  // 差出人の名前とメンションの表示に使う、行の Store のメンバー。画面を開いているあいだ、Store ごとに 1 回読む。
  const members = new SvelteMap<string, Member[]>();
  const asked = new Set<string>();
  $effect(() => {
    for (const r of $view.rows) {
      const id = r.store.id;
      if (asked.has(id)) continue;
      asked.add(id);
      listMembers(id)
        .then((m) => members.set(id, m))
        .catch(() => {});
    }
  });
</script>

<PageHead
  title="受信箱"
  note="同期で届いた変更、メンション、割り当て、上書きされた値"
  {ondrawer}
  {alert}
>
  {#snippet actions()}
    <Button size="sm" disabled={!$canReadAll} onclick={() => void inbox.readAll()}>
      すべて既読にする
    </Button>
  {/snippet}
</PageHead>
<Tabs {tabs} current={$view.tab} onselect={(id) => void inbox.select(id as InboxTab)} />
<div class="content">
  {#if $view.rows.length > 0}
    <ul class="rows">
      {#each $view.rows as row (row.id)}
        <InboxRow {row} members={members.get(row.store.id)} />
      {/each}
    </ul>
    {#if $view.more}
      <div class="more">
        <Button size="sm" disabled={$view.loading} onclick={() => void inbox.more()}>
          さらに読み込む
        </Button>
      </div>
    {/if}
  {:else if !$view.loading}
    <div class="empty">
      <p class="empty-title">{empty.title}</p>
      {#if empty.note}
        <p class="empty-note">{empty.note}</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 8px 24px 40px;
    min-height: 0;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .more {
    display: flex;
    justify-content: center;
    padding: 12px 0;
  }
  .empty {
    padding: 40px 0;
    text-align: center;
  }
  .empty-title {
    margin: 0;
    font-weight: 500;
  }
  .empty-note {
    margin: 6px auto 0;
    max-width: 520px;
    color: var(--muted);
    font-size: 12.5px;
  }
  @container frame (max-width: 860px) {
    .content {
      padding: 8px max(16px, env(safe-area-inset-right, 0px)) 32px
        max(16px, env(safe-area-inset-left, 0px));
    }
  }
</style>
