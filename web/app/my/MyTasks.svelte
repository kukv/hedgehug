<script lang="ts">
  import { cliLs } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import ListBar from "../list/ListBar.svelte";
  import { onLinkClick, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { propText } from "../tree/marks";
  import IdLabel from "../ui/IdLabel.svelte";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import {
    bucketOf,
    dueDay,
    groupMyTasks,
    loginsOf,
    MY_QUERY,
    type MyGroupBy,
    workspaceCount,
  } from "./groups";

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // The filter field holds the whole query: nothing on this screen is fixed.
  let applied = $state(MY_QUERY);
  let by = $state<MyGroupBy>("due");

  // Every store, so no store is asked for. The cache asks again at every tasks.changed and resync.
  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const w = c.cache.watchList({ q: applied, tree: false });
    list = w;
    return () => w.dispose();
  });

  const loading = $derived.by(() => {
    cacheRev.current;
    return list === null || list.loading();
  });
  const error = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []);
  });
  // The date is read again whenever the tasks or the grouping change, so a tab left open over
  // midnight catches up at the next change.
  const grouped = $derived.by(() => {
    const now = new Date();
    return { now, groups: groupMyTasks(tasks, by, now) };
  });

  // The category of a status is in the schema of the store of the task.
  $effect(() => {
    for (const store of new Set(tasks.map((t) => t.store))) {
      c.schema.load(store).catch(() => undefined);
    }
  });
  function categoryOf(store: string, status: string): string | undefined {
    schemaRev.current;
    return c.schema.get(store)?.statuses?.find((s) => s.name === status)?.category;
  }

  const logins = $derived.by(() => {
    storesRev.current;
    return loginsOf(c.stores.stores());
  });
  const count = $derived(
    loading || error ? "" : `${workspaceCount(tasks)} ワークスペース · ${tasks.length} 件`,
  );
  const isDefault = $derived(applied === MY_QUERY);

  const taskHref = (ulid: string) => router.href({ name: "task", ulid });
  function dueText(t: Parameters<typeof propText>[0]): string {
    const d = propText(t, "due");
    return d ? formatDate(d, grouped.now) : "";
  }
  function dueAlert(t: Parameters<typeof propText>[0]): boolean {
    const b = bucketOf(dueDay(t), grouped.now);
    return b === "overdue" || b === "today";
  }

  let groupAnchor = $state<HTMLElement | null>(null);
  const groupItems = [
    { value: "due", label: "期限" },
    { value: "project", label: "プロジェクト" },
  ];
  const groupLabel = $derived(groupItems.find((i) => i.value === by)?.label);
</script>

<div class="my">
  <header class="head">
    <div class="title">
      <Icon name="checklist" size={16} />
      <h1>マイタスク</h1>
    </div>
    {#if logins.length > 0}
      <p class="me">自分 = {logins.join(" ＋ ")}</p>
    {/if}
  </header>

  <ListBar
    base=""
    {applied}
    {count}
    cli={cliLs(applied, false)}
    {error}
    onchange={(q) => (applied = q)}
  >
    {#snippet controls()}
      <button
        type="button"
        class="group-btn"
        aria-haspopup="menu"
        onclick={(e) => (groupAnchor = e.currentTarget)}
      >
        まとめ方: {groupLabel}
      </button>
    {/snippet}
  </ListBar>

  <div class="body">
    {#if error}
      <!-- the bar above tells why -->
    {:else if loading}
      <p class="msg">読み込み中…</p>
    {:else if tasks.length === 0}
      <Empty
        variant="nomatch"
        note={isDefault
          ? "自分が担当で、完了と中止でないタスクはありません"
          : "条件を最初の状態（自分が担当で、完了と中止でない）に戻せます"}
        onClear={isDefault ? undefined : () => (applied = MY_QUERY)}
      />
    {:else}
      {#each grouped.groups as g (g.key)}
        <section class="group" aria-labelledby="my-{g.key}">
          <div class="g-head">
            <h2 id="my-{g.key}" class:alert={g.tone === "alert"}>{g.name}</h2>
            <span class="sub">{g.tasks.length}</span>
          </div>
          <table>
            <tbody>
              {#each g.tasks as t (t.ulid)}
                {@const href = taskHref(t.ulid)}
                <tr>
                  <td class="w-status">
                    <StatusIcon status={t.status} category={categoryOf(t.store, t.status)} />
                  </td>
                  <td class="w-kind"><KindIcon kind={t.kind} /></td>
                  <td class="w-id">
                    <IdLabel id={t.id} ulid={t.ulid} {href} onopen={(e) => onLinkClick(e, href)} />
                  </td>
                  <td class="t-title">
                    <a {href} onclick={(e) => onLinkClick(e, href)}>{t.title}</a>
                  </td>
                  <td class="t-project">{by === "due" ? (t.project?.title ?? "") : ""}</td>
                  <td class="w-due" class:alert={dueAlert(t)}>{dueText(t)}</td>
                  <td class="w-store">
                    <StoreBadge store={t.store} tone={c.stores.tone(t.store)} compact />
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </section>
      {/each}
    {/if}
  </div>
</div>

{#if groupAnchor}
  <Menu
    anchor={groupAnchor}
    label="まとめ方"
    items={groupItems}
    selected={by}
    onselect={(v) => {
      by = v as MyGroupBy;
      groupAnchor = null;
    }}
    onclose={() => (groupAnchor = null)}
  />
{/if}

<style>
  .my {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-12);
    padding: var(--space-16) var(--gutter) 0;
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-10);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .me {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .group-btn {
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink-2);
    font-size: var(--fs-12);
    white-space: nowrap;
    cursor: pointer;
  }
  .group-btn:hover {
    background: var(--hover);
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-20);
    padding: var(--space-8) var(--gutter) var(--space-20);
  }
  .msg {
    margin: 0;
    color: var(--muted);
  }
  .g-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-8);
    margin-bottom: var(--space-4);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  h2.alert,
  .alert {
    color: var(--danger);
  }
  .sub {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  td {
    height: var(--row-h);
    padding: 0 var(--space-8);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .w-status,
  .w-kind {
    width: 2rem;
  }
  .w-id {
    width: 6.5rem;
  }
  .w-due {
    width: 4.5rem;
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--ink-2);
  }
  .w-due.alert {
    color: var(--danger);
    font-weight: 500;
  }
  .w-store {
    width: 9rem;
    text-align: right;
  }
  .t-title {
    min-width: 0;
  }
  .t-title a {
    color: var(--ink);
    text-decoration: none;
  }
  .t-title a:hover {
    text-decoration: underline;
  }
  .t-project {
    max-width: 14rem;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-12);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
