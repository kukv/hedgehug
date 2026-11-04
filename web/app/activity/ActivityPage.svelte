<script lang="ts">
  import { tick, untrack } from "svelte";
  import { authorLabel, groupByDay, type Log, opLabel, trailerLine } from "../../core/activity";
  import { unwrap } from "../../core/api/client";
  import { cliLog } from "../../core/cli";
  import { formatTime } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { focusLost } from "../focus";
  import { type ActivitySync, onLinkClick, router } from "../router";
  import { useManyStores, useSubscribe } from "../svelte-store";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import WorkspacePicker from "./WorkspacePicker.svelte";

  // The filters are the ones of the address (workspaces: ULIDs).
  type Props = { workspaces: string[]; sync: ActivitySync };
  let { workspaces, sync }: Props = $props();

  const PAGE = 100;
  // The most the server gives (the maximum of its limit).
  const MAX = 1000;
  const SYNCS: { value: ActivitySync; label: string }[] = [
    { value: "", label: "すべて" },
    { value: "exclude", label: "人" },
    { value: "only", label: "同期" },
  ];

  const c = core();
  const spaces = useSubscribe(c.stores, () => c.stores.workspaces());
  const manyStores = useManyStores(c.stores);

  let items = $state.raw<Log[] | null>(null);
  let failure = $state<string | null>(null);
  let limit = $state(PAGE);
  // The limit that the commits shown were asked with.
  let shown = $state(PAGE);
  let loading = $state(false);
  let seq = 0;
  let heading: HTMLElement | undefined = $state();
  let feed: HTMLElement | undefined = $state();
  let moreEl: HTMLElement | undefined = $state();
  // How many commits were shown when 「さらに読み込む」 was pressed; null when it was not.
  let askedFrom: number | null = null;

  async function load(): Promise<void> {
    const mine = ++seq;
    const asked = limit;
    loading = true;
    const query: { limit: number; workspace?: string; sync?: "exclude" | "only" } = { limit };
    if (workspaces.length > 0) query.workspace = workspaces.join(",");
    if (sync !== "") query.sync = sync;
    try {
      const list = await unwrap(c.api.GET("/api/activity", { params: { query } }));
      if (mine !== seq) return;
      items = list ?? [];
      shown = asked;
      failure = null;
    } catch (e) {
      if (mine === seq) failure = errorMessage(e);
    } finally {
      if (mine === seq) loading = false;
    }
    if (mine === seq && askedFrom !== null) await settleFocus(askedFrom);
  }

  // The button is disabled while it loads and is gone at the end of the commits, which drops the
  // focus: it goes back to the button, else to the first commit that came, else to the heading.
  async function settleFocus(from: number): Promise<void> {
    askedFrom = null;
    await tick();
    if (!focusLost()) return;
    const rows = feed?.querySelectorAll<HTMLElement>("li.item");
    (moreEl?.querySelector("button") ?? rows?.[from] ?? heading)?.focus();
  }

  function loadMore(): void {
    askedFrom = items?.length ?? 0;
    limit = Math.min(limit + PAGE, MAX);
  }

  // A new request for each change of the filters and of the number asked for; the commits change
  // with a sync (tasks.changed) too.
  // The bare expressions are the reads that make the effect depend on them.
  $effect(() => {
    workspaces.join(",");
    sync;
    limit;
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

  const many = $derived(manyStores.current);
  const days = $derived(groupByDay(items ?? [], new Date()));
  // A full page may have more behind it, up to what the server gives.
  const more = $derived(items !== null && items.length >= shown && shown < MAX);

  // The name of a workspace in the command; its ULID when another workspace (of another Store)
  // that is not chosen has the same name, as the name would take that one in too.
  function nameOf(ulid: string, all: WorkspaceEntry[]): string {
    const w = all.find((x) => x.ulid === ulid);
    if (!w) return ulid;
    const clash = all.some(
      (o) => o.ulid !== ulid && o.name === w.name && !workspaces.includes(o.ulid),
    );
    return clash ? ulid : w.name;
  }
  const command = $derived(cliLog([...new Set(workspaces.map((u) => nameOf(u, spaces.current)))]));

  function go(next: { workspaces?: string[]; sync?: ActivitySync }): void {
    // a load of 「さらに読み込む」 that this takes over does not move the focus
    askedFrom = null;
    limit = PAGE;
    router.replace(
      router.href({
        name: "activity",
        workspaces: next.workspaces ?? workspaces,
        sync: next.sync ?? sync,
      }),
    );
  }
</script>

<div class="activity">
  <header class="head">
    <h1 bind:this={heading} tabindex="-1">アクティビティ</h1>
    <span class="sub">各 Store の git log · 誰がいつ何を変えたか</span>
  </header>

  <div class="bar">
    <WorkspacePicker
      workspaces={spaces.current}
      selected={workspaces}
      onchange={(ulids) => go({ workspaces: ulids })}
    />
    <fieldset class="seg" aria-label="変更の種類">
      {#each SYNCS as s (s.value)}
        <button
          type="button"
          class:on={sync === s.value}
          aria-pressed={sync === s.value}
          onclick={() => go({ sync: s.value })}
        >
          {s.label}
        </button>
      {/each}
    </fieldset>
    <span class="cli"><CliLine {command} /></span>
  </div>

  <div class="body">
    {#if failure}
      <FormError message={failure} />
    {/if}
    {#if items === null}
      {#if !failure}
        <p class="msg">読み込み中…</p>
      {/if}
    {:else if items.length === 0}
      <p class="msg">アクティビティはありません</p>
    {:else}
      <section class="feed" aria-label="変更の一覧" bind:this={feed}>
        {#each days as day (day.key)}
          <section aria-labelledby="day-{day.key}">
            <h2 id="day-{day.key}">{day.name}</h2>
            <ol class="list">
              {#each day.logs as l (`${l.store}/${l.hash}`)}
                {@const who = authorLabel(l)}
                <li class="item" tabindex="-1">
                  <time class="time" datetime={l.date}>{formatTime(l.date)}</time>
                  <Avatar name={l.author.name} />
                  <div class="main">
                    <div class="line">
                      <b>{who}</b>
                      <span class="op">{opLabel(l.op)}</span>
                      <span class="subject">{l.subject}</span>
                      {#if l.task}
                        {@const href = router.href({ name: "task", ulid: l.task })}
                        {#if l.id}
                          <IdLabel
                            id={l.id}
                            ulid={l.task}
                            store={l.store}
                            {href}
                            onopen={(e) => onLinkClick(e, href)}
                          />
                        {:else}
                          <a class="gone" {href} onclick={(e) => onLinkClick(e, href)}>{l.task}</a>
                        {/if}
                      {/if}
                    </div>
                    <code class="trailer">{trailerLine(l)}</code>
                  </div>
                  {#if many}
                    <StoreBadge store={l.store} tone={c.stores.tone(l.store)} compact />
                  {/if}
                </li>
              {/each}
            </ol>
          </section>
        {/each}
      </section>
      {#if more}
        <div class="more" bind:this={moreEl}>
          <Button disabled={loading} onclick={loadMore}>さらに読み込む</Button>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .activity {
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
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-12) var(--gutter);
  }
  .seg {
    display: inline-flex;
    min-width: 0;
    margin: 0;
    border: 0;
    padding: 3px;
    border-radius: var(--radius-8);
    background: var(--hover);
  }
  .seg button {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-10);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .seg button.on {
    background: var(--surface);
    color: var(--ink);
    font-weight: 600;
    box-shadow: var(--shadow-popover);
  }
  .cli {
    display: flex;
    margin-left: auto;
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    max-width: 65rem;
    padding: 0 var(--gutter) var(--space-20);
  }
  .msg {
    margin: 0;
    padding: var(--space-16) 0;
    color: var(--muted);
  }
  h2 {
    margin: 0 0 var(--space-6);
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .feed {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .list {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
    list-style: none;
  }
  .item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-12);
    padding: var(--space-12) var(--space-4);
    border-bottom: 1px solid var(--line);
  }
  .time {
    flex: 0 0 auto;
    width: 3.5em;
    padding-top: var(--space-4);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
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
    gap: var(--space-6);
    line-height: 1.6;
  }
  .op {
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .subject {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .gone {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    text-decoration: none;
  }
  .trailer {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    overflow-wrap: anywhere;
  }
  .more {
    display: flex;
    justify-content: center;
  }
</style>
