<script lang="ts">
  import { untrack } from "svelte";
  import { cliSprintClose } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    type CloseTo,
    closeLabel,
    closeToValue,
    inRankOrder,
    nextSprint,
    ptLabel,
    queryValue,
    type Sprint,
    sprintDates,
    sumPoints,
  } from "../../core/planning";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { plansWritten } from "./plans.svelte";

  type Props = { entry: WorkspaceEntry; sprint: Sprint; sprints: Sprint[]; onclose: () => void };
  let { entry, sprint, sprints, onclose }: Props = $props();

  const c = core();
  const uid = $props.id();
  const did = useDisplayId(c.stores);
  const cacheRev = useSubscribe(c.cache, () => ({}));

  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const h = c.cache.watchList({
      q: `workspace:${entry.ulid} sprint:${queryValue(sprint.name)} status:todo,doing`,
      tree: false,
      store: entry.store,
    });
    list = h;
    return () => h.dispose();
  });
  const open = $derived.by(() => {
    cacheRev.current;
    return inRankOrder((list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []));
  });
  const loading = $derived.by(() => {
    cacheRev.current;
    return list === null || list.loading();
  });

  const next = $derived(nextSprint(sprints.filter((s) => s.ulid !== sprint.ulid)));
  const others = $derived(
    sprints.filter((s) => s.ulid !== sprint.ulid && s.ulid !== next?.ulid && s.state !== "closed"),
  );
  let to = $state.raw<CloseTo>(untrack(() => (next ? { kind: "next" } : { kind: "backlog" })));
  const value = $derived(open.length > 0 ? closeToValue(to) : undefined);

  let busy = $state(false);
  let failure = $state<string | null>(null);

  async function submit() {
    if (busy || loading) return;
    busy = true;
    failure = null;
    try {
      const r = await c.api.closeSprint(sprint.ulid, value ? { to: value } : {}, {
        store: entry.store,
      });
      plansWritten();
      const moved = r.moved
        ? ` · ${r.moved} 件を${r.to === "backlog" ? "バックログ" : r.to}へ`
        : "";
      toast({ message: `スプリント ${sprint.name} を閉じました${moved}` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open
  title={`${sprint.name} を閉じる`}
  width={760}
  cli={cliSprintClose(sprint.name, value, entry.store)}
  {onclose}
>
  <div class="form">
    <p class="meta">
      <span class="mono">{sprintDates(sprint)}</span>
      {sprint.goal ? ` · ゴール: ${sprint.goal}` : ""}
    </p>
    <div class="stats">
      <div class="stat done"><span>完了</span><b>{sprint.done} 件</b></div>
      <div class="stat">
        <span>未完了</span><b>{open.length} 件 · {ptLabel(sumPoints(open))}</b>
      </div>
    </div>
    {#if open.length > 0}
      <fieldset>
        <legend>未完了の {open.length} 件の移動先</legend>
        <div class="choices">
          <label class="choice" class:on={to.kind === "next"} class:off={!next}>
            <input
              type="radio"
              name="{uid}-to"
              checked={to.kind === "next"}
              disabled={!next}
              onchange={() => (to = { kind: "next" })}
            >
            {next ? `次のスプリント（${next.name} · ${next.state}）` : "次のスプリントはありません"}
          </label>
          <label class="choice" class:on={to.kind === "backlog"}>
            <input
              type="radio"
              name="{uid}-to"
              checked={to.kind === "backlog"}
              onchange={() => (to = { kind: "backlog" })}
            >
            バックログ
          </label>
          {#each others as s (s.ulid)}
            <label class="choice" class:on={to.kind === "sprint" && to.name === s.name}>
              <input
                type="radio"
                name="{uid}-to"
                checked={to.kind === "sprint" && to.name === s.name}
                onchange={() => (to = { kind: "sprint", name: s.name })}
              >
              {s.name}（{s.state}）
            </label>
          {/each}
        </div>
        <ul>
          {#each open as t (t.ulid)}
            <li>
              <span class="mono id">{did.current(t.store, t.id)}</span>
              <span class="title">{t.title}</span>
            </li>
          {/each}
        </ul>
      </fieldset>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy || loading} onclick={() => void submit()}>
      {closeLabel(open.length, to, next)}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .meta {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .stats {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-10);
  }
  .stat {
    display: flex;
    flex: 1 1 140px;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-8);
    background: var(--sunken);
  }
  .stat span {
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .stat b {
    font-size: var(--fs-19);
    font-weight: 600;
  }
  fieldset {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    margin: 0;
    padding: 0;
    border: 0;
  }
  legend {
    padding: 0 0 var(--space-4);
    font-weight: 600;
  }
  .choices {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }
  .choice {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    cursor: pointer;
  }
  .choice.on {
    border-color: var(--accent);
    background: var(--accent-weak);
  }
  .choice.off {
    color: var(--muted);
    cursor: default;
  }
  ul {
    max-height: 240px;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-10);
    align-items: center;
    min-height: 34px;
    border-top: 1px solid var(--line);
  }
  .id {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
