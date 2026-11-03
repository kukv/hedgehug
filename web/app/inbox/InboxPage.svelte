<script lang="ts">
  import { tick, untrack } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliInbox } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    filterByTab,
    formatWhen,
    INBOX_TABS,
    type InboxTab,
    type Notification,
    notificationText,
    overwrittenField,
    sortNotifications,
    tabCounts,
    taskOf,
  } from "../../core/notifications";
  import { core } from "../context";
  import { focusLost } from "../focus";
  import { onLinkClick, router } from "../router";
  import { useManyStores, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";

  // The tab is in the address (?tab=), so a tab opened in a new browser tab is the same tab.
  let { tab }: { tab: InboxTab } = $props();

  const c = core();
  const unread = useSubscribe(c.conn, () => c.conn.unread);
  const manyStores = useManyStores(c.stores);

  // Every store at once: the server reads each inbox when no store is named.
  let items = $state.raw<Notification[] | null>(null);
  let failure = $state<string | null>(null);
  // The items (by ULID) and "all" that have a request on their way.
  let busy = $state<string[]>([]);
  let seq = 0;
  let heading: HTMLElement | undefined = $state();
  let list: HTMLElement | undefined = $state();

  async function load(): Promise<void> {
    const mine = ++seq;
    try {
      const list = await unwrap(
        c.api.GET("/api/notifications", { params: { query: { all: true } } }),
      );
      if (mine !== seq) return;
      items = list ?? [];
      failure = null;
    } catch (e) {
      if (mine === seq) failure = errorMessage(e);
    }
  }

  // The count of unread changes when the item is read in another tab or by `hh inbox read`; the
  // list changes with a sync (tasks.changed) and a resync.
  $effect(() => {
    unread.current;
    untrack(() => void load());
  });
  $effect(() => {
    const offs = [
      c.conn.on("resync", () => void load()),
      c.conn.on("tasks.changed", () => void load()),
    ];
    return () => {
      for (const off of offs) off();
      // an answer that comes after the page is gone is dropped
      seq++;
    };
  });

  const sorted = $derived(sortNotifications(items ?? []));
  const counts = $derived(tabCounts(items ?? []));
  const shown = $derived(filterByTab(sorted, tab));
  const many = $derived(manyStores.current);
  const now = $derived.by(() => {
    items;
    return new Date();
  });
  const tabLabel = $derived(INBOX_TABS.find((t) => t.tab === tab)?.label ?? "");

  const tabItems = $derived(
    INBOX_TABS.map((t) => ({
      label: t.label,
      href: router.href({ name: "inbox", tab: t.tab }),
      current: tab === t.tab,
      count: counts[t.tab],
    })),
  );

  function text(n: Notification) {
    return notificationText(n, {
      workspaceName: (ulid) => c.stores.workspace(n.store, ulid)?.name,
    });
  }

  // run sends a request for the item (or for "all"), shows what it said when it fails, and reads
  // the list again. The button the user pressed is disabled meanwhile, and may be gone after: when
  // that drops the focus, it goes to the row of the item, and to the heading when it is "all" (or
  // the row is gone).
  async function run(key: string, fn: () => Promise<void>): Promise<void> {
    busy = [...busy, key];
    try {
      await fn();
    } catch (e) {
      toast({ message: errorMessage(e) });
    } finally {
      busy = busy.filter((k) => k !== key);
    }
    await load();
    await tick();
    if (!focusLost()) return;
    const row =
      key === "all" ? undefined : list?.querySelector<HTMLElement>(`[data-ulid="${key}"]`);
    (row ?? heading)?.focus();
  }

  const read = (n: Notification) =>
    run(n.ulid, async () => {
      await unwrap(
        c.api.PATCH("/api/notifications/{id}", {
          params: { path: { id: n.ulid } },
          body: { read: true },
        }),
      );
    });

  const readAll = () =>
    run("all", async () => {
      await unwrap(c.api.POST("/api/notifications/read", { body: { all: true } }));
    });

  const revert = (n: Notification) =>
    run(n.ulid, async () => {
      const r = await unwrap(
        c.api.POST("/api/notifications/{id}/revert", { params: { path: { id: n.ulid } } }),
      );
      const id = taskOf(n)?.id || r.task.id;
      toast({ message: `${id} の ${overwrittenField(n)} を自分の値に戻しました` });
    });

  // Opening the task reads the item first (⌘/Ctrl-click and the middle button too).
  function open(e: MouseEvent, n: Notification, href: string): void {
    if (!n.read && !isBusy(n)) void read(n);
    onLinkClick(e, href);
  }

  // The middle button opens the task in a new tab by itself, and reads the item too.
  function openAux(e: MouseEvent, n: Notification): void {
    if (e.button === 1 && !n.read && !isBusy(n)) void read(n);
  }

  const isBusy = (n: Notification) => busy.includes(n.ulid);
</script>

<div class="inbox">
  <header class="head">
    <h1 bind:this={heading} tabindex="-1">通知</h1>
    <span class="sub"
      >同期で届いた変更から · 自分の書き込みは通知しません · 既読はこのマシンに保存</span
    >
    <span class="end">
      <Button
        size="sm"
        disabled={counts.all === 0 || busy.includes("all")}
        onclick={() => void readAll()}
      >
        すべて既読にする
      </Button>
    </span>
  </header>

  <Tabs label="通知の種類" items={tabItems} />

  <div class="body">
    {#if failure}
      <FormError message={failure} />
    {/if}
    {#if items === null}
      {#if !failure}
        <p class="msg">読み込み中…</p>
      {/if}
    {:else if shown.length === 0}
      <p class="msg">{tab === "all" ? "通知はありません" : `${tabLabel}の通知はありません`}</p>
    {:else}
      <ul class="list" bind:this={list}>
        {#each shown as n (n.ulid)}
          {@const task = taskOf(n)}
          {@const href = task ? router.href({ name: "task", ulid: task.ulid }) : ""}
          {@const t = text(n)}
          <li class="item" class:unread={!n.read} data-ulid={n.ulid} tabindex="-1">
            {#if n.read}
              <span class="dot"></span>
            {:else}
              <span class="dot" role="img" aria-label="未読"></span>
            {/if}
            <div class="main">
              <div class="line">
                <span class="kind" data-kind={n.kind}>{t.label}</span>
                {#if many}
                  <StoreBadge store={n.store} tone={c.stores.tone(n.store)} compact />
                {/if}
                {#if task}
                  <IdLabel
                    id={task.id}
                    ulid={task.ulid}
                    {href}
                    onopen={(e) => open(e, n, href)}
                    onaux={(e) => openAux(e, n)}
                  />
                  <a
                    class="title"
                    {href}
                    onclick={(e) => open(e, n, href)}
                    onauxclick={(e) => openAux(e, n)}
                    >{task.title}</a
                  >
                {/if}
                <time class="when" datetime={n.time}>{formatWhen(n.time, now)}</time>
              </div>
              <p class="text">{t.text}</p>
              {#if t.mine}
                <div class="diff"><del>{t.mine}</del></div>
              {/if}
              {#if !n.read}
                <div class="actions">
                  {#if n.kind === "overwritten" && task}
                    <Button
                      size="sm"
                      variant="confirm"
                      disabled={isBusy(n)}
                      onclick={() => void revert(n)}
                    >
                      自分の値に戻す
                    </Button>
                    <Button size="sm" disabled={isBusy(n)} onclick={() => void read(n)}
                      >このままにする</Button
                    >
                  {:else}
                    <Button size="sm" disabled={isBusy(n)} onclick={() => void read(n)}
                      >既読にする</Button
                    >
                  {/if}
                </div>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
    <div class="cli"><CliLine command={cliInbox({ all: true })} /></div>
  </div>
</div>

<style>
  .inbox {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-12);
    padding: var(--space-16) var(--gutter) 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .sub {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .end {
    margin-left: auto;
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-8) var(--gutter) var(--space-20);
  }
  .msg {
    margin: 0;
    padding: var(--space-16) 0;
    color: var(--muted);
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0;
    padding: 0;
    list-style: none;
    max-width: 65rem;
  }
  .item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-12);
    padding: var(--space-12);
    border-radius: var(--radius-10);
  }
  .item.unread {
    background: var(--accent-faint);
  }
  .dot {
    flex: 0 0 auto;
    width: 8px;
    height: 8px;
    margin-top: 6px;
    border-radius: var(--radius-pill);
    background: transparent;
  }
  .unread .dot {
    background: var(--accent);
  }
  .main {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
  }
  .kind {
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .kind[data-kind="overwritten"] {
    color: var(--warning);
  }
  .kind[data-kind="mention"] {
    color: var(--accent-ink);
  }
  .title {
    min-width: 0;
    overflow: hidden;
    color: var(--ink-2);
    text-decoration: none;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title:hover {
    text-decoration: underline;
  }
  .when {
    margin-left: auto;
    color: var(--muted);
    font-size: var(--fs-12);
    white-space: nowrap;
  }
  .text {
    margin: 0;
    line-height: 1.65;
  }
  .diff {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  del {
    padding: 2px var(--space-8);
    border-radius: var(--radius-4);
    background: var(--danger-weak);
    color: var(--danger-ink);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    margin-top: 2px;
  }
  .cli {
    display: flex;
  }
</style>
