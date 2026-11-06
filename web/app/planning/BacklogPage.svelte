<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import {
    BACKLOG_QUERY,
    inRankOrder,
    placed,
    ptLabel,
    type Sprint,
    sprintsInOrder,
    sumPoints,
  } from "../../core/planning";
  import type { SchemaView } from "../../core/schema";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { useReadOnlySentence } from "../read-only";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import { type Edge, listDrop } from "./dnd";
  import PlanRow from "./PlanRow.svelte";
  import { usePlans } from "./plans.svelte";
  import SprintPanel from "./SprintPanel.svelte";

  const SHOWN = 100;

  const isContainer = (schema: SchemaView | undefined, kind: string): boolean =>
    schema?.kinds?.find((k) => k.name === kind)?.role === "container";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const offSentence = useReadOnlySentence(c.stores, () => entry.store);
  const locked = $derived(offSentence.current !== null);
  const plans = usePlans(() => ({ store: entry.store, name: entry.name }));

  $effect(() => {
    void c.schema.load(entry.store).catch(() => undefined);
  });
  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(entry.store);
  });

  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const h = c.cache.watchList({
      q: `workspace:${entry.ulid} ${BACKLOG_QUERY}`,
      tree: false,
      store: entry.store,
    });
    list = h;
    return () => h.dispose();
  });
  // The projects hold work; they are not work to put in a sprint.
  const tasks = $derived.by(() => {
    cacheRev.current;
    const all = (list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []);
    return inRankOrder(all.filter((t) => !isContainer(schema, t.kind)));
  });
  const error = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });

  let all = $state(false);
  const shown = $derived(all ? tasks : tasks.slice(0, SHOWN));
  const open = $derived(sprintsInOrder(plans.sprints).filter((s) => s.state !== "closed"));
  const moves = $derived(open.map((s) => ({ value: s.ulid, label: `${s.name} に入れる` })));

  const run = async (write: () => Promise<unknown>) => {
    try {
      await write();
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  };

  const taskOf = (ulid: string) => c.cache.task(ulid);
  const intoSprint = (ulid: string, sprint: Sprint | null) => {
    const t = taskOf(ulid);
    if (t) void run(() => patchTask(c, t, { sprint: sprint?.ulid ?? "" }));
  };

  // A rank says the place among all the tasks of the list. A task out of a sprint leaves it first.
  async function rankTo(t: TaskView, at: number) {
    if (t.sprint) await patchTask(c, t, { sprint: "" });
    const order = placed(
      tasks.map((x) => x.ulid),
      t.ulid,
      at,
    );
    const r = await c.api.rankTask(t.ulid, { order }, { store: t.store });
    c.cache.applyWrite({ task: r.task, commit: r.commit });
  }

  function dropOnRow(onto: string, dragged: string, edge: Edge) {
    const t = taskOf(dragged);
    if (!t) return;
    const others = tasks.filter((x) => x.ulid !== dragged);
    const i = others.findIndex((x) => x.ulid === onto);
    void run(() => rankTo(t, edge === "top" ? i : i + 1));
  }

  let over = $state(false);
  function dropOnList(dragged: string, inner: boolean) {
    const t = taskOf(dragged);
    if (!inner && t?.sprint) void run(() => rankTo(t, tasks.length));
  }
</script>

<div class="page">
  <section class="backlog" aria-labelledby="backlog-h">
    <div class="head">
      <h2 id="backlog-h">バックログ</h2>
      <span class="meta">
        スプリント未割り当て · todo / doing · rank 順 · {tasks.length} 件 ·
        {ptLabel(sumPoints(tasks))}
      </span>
    </div>
    {#if error}
      <p class="msg" role="alert">{errorMessage(error)}</p>
    {/if}
    <ul
      class:over
      aria-label="バックログのタスク"
      {@attach listDrop(() => ({ onOver: (o) => (over = o), onDrop: dropOnList }))}
    >
      {#each shown as t (t.ulid)}
        <PlanRow
          task={t}
          {schema}
          {locked}
          {moves}
          onmove={(v) => intoSprint(t.ulid, open.find((s) => s.ulid === v) ?? null)}
          ondrop={(dragged, edge) => dropOnRow(t.ulid, dragged, edge)}
        />
      {:else}
        <li class="empty">バックログは空です</li>
      {/each}
    </ul>
    {#if !all && tasks.length > SHOWN}
      <button type="button" class="link" onclick={() => (all = true)}>
        ほか {tasks.length - SHOWN} 件を表示
      </button>
    {/if}
  </section>

  <aside aria-label="スプリント">
    {#each open as s (s.ulid)}
      <SprintPanel
        {entry}
        sprint={s}
        sprints={plans.sprints}
        {schema}
        {locked}
        ondrop={(u) => intoSprint(u, s)}
        onbacklog={(u) => intoSprint(u, null)}
      />
    {:else}
      <p class="none">
        {plans.loaded ? "planned か active のスプリントはありません" : "読み込み中…"}
      </p>
    {/each}
  </aside>
</div>

<style>
  .page {
    box-sizing: border-box;
    height: 100%;
    overflow-y: auto;
    display: grid;
    grid-template-columns: minmax(0, 3fr) minmax(280px, 2fr);
    gap: var(--space-16);
    align-items: start;
    padding: var(--space-16) var(--gutter) var(--space-28);
  }
  @media (max-width: 900px) {
    .page {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-10);
    align-items: baseline;
    padding-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
    font-weight: 600;
  }
  .meta {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  ul {
    min-height: 80px;
    margin: 0;
    padding: 0;
    list-style: none;
    border-radius: var(--radius-8);
  }
  ul.over {
    outline: 1.5px solid var(--accent);
  }
  aside {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .empty,
  .none {
    margin: 0;
    padding: var(--space-12);
    color: var(--muted);
  }
  .msg {
    color: var(--danger);
  }
  .link {
    margin-top: var(--space-8);
    border: 0;
    background: transparent;
    color: var(--accent);
    cursor: pointer;
  }
</style>
