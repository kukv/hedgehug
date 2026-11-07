<script lang="ts">
  import { inRankOrder, ptLabel, queryValue, type Sprint, sprintDates } from "../../core/planning";
  import type { SchemaView } from "../../core/schema";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import { openPlanDialog } from "./dialogs-state.svelte";
  import { listDrop } from "./dnd";
  import PlanRow from "./PlanRow.svelte";

  const SHOWN = 5;

  type Props = {
    entry: WorkspaceEntry;
    sprint: Sprint;
    sprints: Sprint[];
    schema: SchemaView | undefined;
    locked: boolean;
    ondrop: (ulid: string) => void;
    onbacklog: (ulid: string) => void;
  };
  let { entry, sprint, sprints, schema, locked, ondrop, onbacklog }: Props = $props();

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const h = c.cache.watchList({
      q: `workspace:${entry.ulid} sprint:${queryValue(sprint.name)} -status:done,canceled`,
      tree: false,
      store: entry.store,
    });
    list = h;
    return () => h.dispose();
  });
  const tasks = $derived.by(() => {
    cacheRev.current;
    return inRankOrder((list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []));
  });
  let all = $state(false);
  const shown = $derived(all ? tasks : tasks.slice(0, SHOWN));
  const board = $derived(
    router.href({ name: "workspace", store: entry.store, ulid: entry.ulid, tab: "sprints" }),
  );
  let over = $state(false);
  const moves = [{ value: "backlog", label: "バックログに戻す" }];
</script>

<section
  class="panel"
  class:over
  aria-label={sprint.name}
  {@attach listDrop(() => ({
    onOver: (o) => (over = o),
    onDrop: (u) => {
      if (c.cache.task(u)?.sprint?.ulid !== sprint.ulid) ondrop(u);
    },
  }))}
>
  <div class="head">
    <span class="state" data-state={sprint.state}>{sprint.state}</span>
    <h3>{sprint.name}</h3>
    <span class="dates">{sprintDates(sprint)}</span>
  </div>
  {#if sprint.goal}
    <p class="goal">ゴール: {sprint.goal}</p>
  {/if}
  <div class="sum">
    <b>{ptLabel(sprint.points)}</b>
    <span>{sprint.tasks} 件</span>
    {#if sprint.state === "active"}
      <a href={board} onclick={(e) => onLinkClick(e, board)}>カンバンを開く</a>
    {:else}
      <Button
        size="sm"
        disabled={locked}
        onclick={() => openPlanDialog({ kind: "sprint-start", entry, sprint, sprints })}
      >
        開始する
      </Button>
    {/if}
  </div>
  <ul>
    {#each shown as t (t.ulid)}
      <PlanRow task={t} {schema} {locked} {moves} onmove={() => onbacklog(t.ulid)} />
    {/each}
  </ul>
  {#if !all && tasks.length > SHOWN}
    <button type="button" class="link" onclick={() => (all = true)}>
      ほか {tasks.length - SHOWN} 件を表示
    </button>
  {/if}
</section>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-12);
    border-radius: var(--radius-10);
    background: var(--sunken);
  }
  .panel.over {
    outline: 1.5px solid var(--accent);
    background: var(--accent-weak);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    align-items: center;
  }
  h3 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .state {
    padding: 0 var(--space-6);
    border-radius: 999px;
    background: var(--surface);
    font-size: var(--fs-11);
  }
  .state[data-state="active"] {
    background: var(--accent-weak);
  }
  .dates {
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .goal {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .sum {
    display: flex;
    gap: var(--space-10);
    align-items: center;
  }
  .sum b {
    font-size: var(--fs-15);
    font-weight: 600;
  }
  .sum span {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .sum :global(:last-child) {
    margin-left: auto;
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .link {
    border: 0;
    background: transparent;
    color: var(--accent);
    cursor: pointer;
  }
</style>
