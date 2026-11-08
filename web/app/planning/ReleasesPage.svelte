<script lang="ts">
  import { categoryOf } from "../../core/categories";
  import { cliReleaseNotes } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    groupByKind,
    queryValue,
    RELEASE_STATES,
    type Release,
    type ReleaseState,
    releaseDue,
  } from "../../core/planning";
  import { kindColorOf, kindIconOf, kindRoleOf, statusColorOf } from "../../core/schema-edit";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { onLinkClick, router } from "../router";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import { copyText } from "../ui/clipboard";
  import { openPlanDialog } from "./dialogs-state.svelte";
  import { plansWritten, usePlans } from "./plans.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const c = core();
  const did = useDisplayId(c.stores);
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const plans = usePlans(() => ({ store: entry.store, name: entry.name }));
  const offSentence = useReadOnlySentence(c.stores, () => entry.store);
  const off = $derived(offSentence.current);

  $effect(() => {
    void c.schema.load(entry.store).catch(() => undefined);
  });
  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(entry.store);
  });

  let picked = $state<string | null>(null);
  const shown = $derived(
    plans.releases.find((r) => r.ulid === picked) ??
      plans.releases.find((r) => r.state === "unreleased") ??
      plans.releases[0],
  );

  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const name = shown?.name;
    if (!name) {
      list = null;
      return;
    }
    const h = c.cache.watchList({
      q: `workspace:${entry.ulid} release:${queryValue(name)}`,
      tree: false,
      store: entry.store,
    });
    list = h;
    return () => h.dispose();
  });
  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []);
  });
  const groups = $derived(
    groupByKind(
      tasks,
      (schema?.kinds ?? []).map((k) => k.name),
    ),
  );
  const counts = $derived.by(() => {
    const n = { done: 0, doing: 0, todo: 0 };
    for (const t of tasks) {
      const cat = schema ? categoryOf(schema, t.status) : "todo";
      if (cat === "done" || cat === "doing" || cat === "todo") n[cat]++;
    }
    return n;
  });

  // The notes follow the tasks: a task done elsewhere is a new line in them.
  let notes = $state("");
  let notesError = $state<string | null>(null);
  $effect(() => {
    const r = shown;
    void tasks;
    if (!r) return;
    let live = true;
    c.api
      .getReleaseNotes(r.ulid, { store: entry.store })
      .then((v) => {
        if (!live) return;
        notes = v.notes;
        notesError = null;
      })
      .catch((e: unknown) => {
        if (live) notesError = errorMessage(e);
      });
    return () => {
      live = false;
    };
  });

  async function setState(r: Release, state: ReleaseState) {
    try {
      await c.api.updateRelease(r.ulid, { state }, { store: entry.store });
      plansWritten();
      toast({ message: `リリース ${r.name} を ${state} にしました` });
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  async function copyNotes() {
    toast({
      message: (await copyText(notes))
        ? "リリースノートをコピーしました"
        : "コピーできませんでした",
    });
  }
  const percent = (r: Release) => (r.tasks > 0 ? Math.round((r.done / r.tasks) * 100) : 0);
  const taskHref = (ulid: string) => router.href({ name: "task", ulid });
</script>

<div class="page">
  <section class="rels" aria-label="リリースの一覧">
    {#each plans.releases as r (r.ulid)}
      <button
        type="button"
        class="rel"
        aria-current={r.ulid === shown?.ulid ? "true" : undefined}
        onclick={() => (picked = r.ulid)}
      >
        <span class="top">
          <b>{r.name}</b>
          <span class="state" data-state={r.state}>{r.state}</span>
          <span class="mono">{r.date?.slice(5).replace("-", "/") ?? "—"}</span>
        </span>
        <span class="top">
          <span class="bar"><span style:width="{percent(r)}%"></span></span>
          <span class="mono">{r.done}/{r.tasks}</span>
        </span>
      </button>
    {:else}
      <p class="none">{plans.loaded ? "リリースはありません" : "読み込み中…"}</p>
    {/each}
  </section>

  {#if plans.error}
    <p class="msg" role="alert">{errorMessage(plans.error)}</p>
  {/if}

  {#if shown}
    {@const r = shown}
    <section class="detail" aria-labelledby="rel-h">
      <div class="head">
        <h2 id="rel-h">{r.name}</h2>
        <span class="state" data-state={r.state}>{r.state}</span>
        <span class="due">{releaseDue(r.date, new Date())}</span>
        <span class="acts">
          <Button
            size="sm"
            disabled={off !== null}
            why={off ?? undefined}
            onclick={() => openPlanDialog({ kind: "release-edit", entry, release: r })}
          >
            編集
          </Button>
          {#each RELEASE_STATES.filter((s) => s !== r.state) as s (s)}
            <Button size="sm" disabled={off !== null} onclick={() => void setState(r, s)}>
              {s}
              にする
            </Button>
          {/each}
          <Button
            size="sm"
            disabled={off !== null}
            onclick={() => openPlanDialog({ kind: "release-rm", entry, release: r })}
          >
            削除
          </Button>
        </span>
      </div>
      <p class="progress">
        <b>{r.done}</b>
        / {r.tasks} 完了
        <span class="mono">done {counts.done} · doing {counts.doing} · todo {counts.todo}</span>
      </p>
      <div class="cols">
        <div class="tasks">
          {#each groups as g (g.kind)}
            <div>
              <h3>
                <KindIcon
                  kind={g.kind}
                  role={kindRoleOf(schema, g.kind)}
                  icon={kindIconOf(schema, g.kind)}
                  color={kindColorOf(schema, g.kind)}
                  size={13}
                />{g.kind}<span class="mono">{g.tasks.length}</span>
              </h3>
              {#each g.tasks as t (t.ulid)}
                {@const href = taskHref(t.ulid)}
                <a {href} onclick={(e) => onLinkClick(e, href)}>
                  <StatusIcon
                    status={t.status}
                    category={schema ? categoryOf(schema, t.status) : undefined}
                    color={statusColorOf(schema, t.status)}
                    size={14}
                  />
                  <span class="mono">{did.current(t.store, t.id)}</span>
                  <span class="title">{t.title}</span>
                </a>
              {/each}
            </div>
          {:else}
            <p class="none">タスクはありません</p>
          {/each}
        </div>
        <aside aria-labelledby="notes-h">
          <div class="nhead">
            <h3 id="notes-h">リリースノート</h3>
            <Button size="sm" disabled={notes === ""} onclick={() => void copyNotes()}>
              Markdown をコピー
            </Button>
          </div>
          {#if notesError}
            <p class="msg" role="alert">{notesError}</p>
          {:else}
            <pre>{notes}</pre>
          {/if}
          <CliLine command={cliReleaseNotes(r.name, entry.store)} />
        </aside>
      </div>
    </section>
  {/if}
</div>

<style>
  .page {
    box-sizing: border-box;
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    padding: var(--space-16) var(--gutter) var(--space-28);
  }
  .rels {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }
  .rel {
    display: flex;
    flex: 0 1 240px;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-10) var(--space-12);
    border: 0;
    border-radius: var(--radius-10);
    background: var(--sunken);
    text-align: left;
    cursor: pointer;
  }
  .rel[aria-current="true"] {
    outline: 1.5px solid var(--accent);
    background: var(--accent-weak);
  }
  .top {
    display: flex;
    gap: var(--space-8);
    align-items: center;
  }
  .top .mono:last-child {
    margin-left: auto;
  }
  .bar {
    flex: 1;
    height: 6px;
    overflow: hidden;
    border-radius: 3px;
    background: var(--line);
  }
  .bar span {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .state {
    padding: 0 var(--space-6);
    border-radius: 999px;
    background: var(--surface);
    font-size: var(--fs-11);
  }
  .detail {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-10);
    align-items: center;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-17);
    font-weight: 600;
  }
  .due {
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
    margin-left: auto;
  }
  .progress {
    display: flex;
    gap: var(--space-10);
    align-items: baseline;
    margin: 0;
  }
  .progress b {
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .cols {
    display: grid;
    grid-template-columns: minmax(0, 3fr) minmax(280px, 2fr);
    gap: var(--space-16);
    align-items: start;
  }
  @media (max-width: 900px) {
    .cols {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .tasks {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  h3 {
    display: flex;
    gap: var(--space-6);
    align-items: center;
    margin: 0 0 var(--space-4);
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .tasks a {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    min-height: 34px;
    border-top: 1px solid var(--line);
    color: var(--ink);
    text-decoration: none;
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  aside {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-10);
  }
  .nhead {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    justify-content: space-between;
  }
  pre {
    min-height: 60px;
    margin: 0;
    overflow-x: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    line-height: 1.7;
    white-space: pre-wrap;
  }
  .msg {
    margin: 0;
    color: var(--danger);
  }
  .none {
    margin: 0;
    color: var(--muted);
  }
</style>
