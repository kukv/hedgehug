<script lang="ts">
  import { errorMessage } from "../../core/messages";
  import {
    daysLeft,
    ptLabel,
    queryValue,
    type Sprint,
    sprintDates,
    sprintsInOrder,
  } from "../../core/planning";
  import type { WorkspaceEntry } from "../../core/stores";
  import type { KanbanSettings } from "../../core/views";
  import { core } from "../context";
  import KanbanView from "../kanban/KanbanView.svelte";
  import { useReadOnlySentence } from "../read-only";
  import SprintBurndown from "../reports/SprintBurndown.svelte";
  import { combine, NO_CANCELED, withoutTerm } from "../tree/filter";
  import Button from "../ui/Button.svelte";
  import { openPlanDialog } from "./dialogs-state.svelte";
  import { usePlans } from "./plans.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const c = core();
  const plans = usePlans(() => ({ store: entry.store, name: entry.name }));
  const offSentence = useReadOnlySentence(c.stores, () => entry.store);
  const off = $derived(offSentence.current);

  const sprints = $derived(sprintsInOrder(plans.sprints));
  const active = $derived(sprints.filter((s) => s.state === "active"));
  let picked = $state<string | null>(null);
  const shown = $derived(active.find((s) => s.ulid === picked) ?? active[0]);

  // The board is the one of the kanban tab, narrowed to the sprint.
  let settings = $state.raw<KanbanSettings>({ columns: "category", swimlane: "assignee" });
  let filter = $state(NO_CANCELED);
  const term = $derived(
    active.length === 1 || !shown ? "sprint:current" : `sprint:${queryValue(shown.name)}`,
  );
  const open = (kind: "sprint-edit" | "sprint-rm", sprint: Sprint) =>
    openPlanDialog({ kind, entry, sprint });
  const withList = (kind: "sprint-start" | "sprint-close", sprint: Sprint) =>
    openPlanDialog({ kind, entry, sprint, sprints: plans.sprints });
</script>

<div class="page">
  {#if plans.error}
    <p class="msg" role="alert">{errorMessage(plans.error)}</p>
  {/if}

  {#if shown}
    <section class="now" aria-label="{shown.name} の状況">
      <div class="what">
        {#if active.length > 1}
          <fieldset class="pick">
            <legend class="sr">active なスプリント</legend>
            {#each active as s (s.ulid)}
              <button
                type="button"
                aria-pressed={s.ulid === shown.ulid}
                onclick={() => (picked = s.ulid)}
              >
                {s.name}
              </button>
            {/each}
          </fieldset>
        {/if}
        <h2>{shown.goal || shown.name}</h2>
        <span class="dates">{shown.name} · {sprintDates(shown)} · {shown.tasks} 件</span>
      </div>
      <div class="stat">
        <span>残り</span><b>{shown.end ? daysLeft(shown.end, new Date()) : "—"} 日</b>
      </div>
      {#key shown.ulid}
        <SprintBurndown {entry} sprint={shown} />
      {/key}
      <Button
        disabled={off !== null}
        why={off ?? undefined}
        onclick={() => withList("sprint-close", shown)}
      >
        スプリントを閉じる…
      </Button>
    </section>
    <div class="board">
      <KanbanView
        view={{ kind: "workspace", store: entry.store, ulid: entry.ulid }}
        query={combine(term, filter)}
        onquery={(q) => (filter = withoutTerm(q, term).trim())}
        {settings}
        onsettings={(s) => (settings = s)}
      />
    </div>
  {/if}

  <section class="list" aria-labelledby="sprints-h">
    <h2 id="sprints-h">スプリントの一覧</h2>
    <table>
      <thead>
        <tr>
          <th scope="col">名前</th>
          <th scope="col">状態</th>
          <th scope="col">期間</th>
          <th scope="col">タスク</th>
          <th scope="col">points</th>
          <th scope="col"><span class="sr">操作</span></th>
        </tr>
      </thead>
      <tbody>
        {#each sprints as s (s.ulid)}
          <tr>
            <th scope="row">{s.name}</th>
            <td><span class="state" data-state={s.state}>{s.state}</span></td>
            <td class="mono">{sprintDates(s)}</td>
            <td class="mono">{s.done}/{s.tasks}</td>
            <td class="mono">{ptLabel(s.points)}</td>
            <td class="acts">
              {#if s.state === "planned"}
                <Button
                  size="sm"
                  disabled={off !== null}
                  onclick={() => withList("sprint-start", s)}
                >
                  開始する
                </Button>
              {:else if s.state === "active"}
                <Button
                  size="sm"
                  disabled={off !== null}
                  onclick={() => withList("sprint-close", s)}
                >
                  閉じる
                </Button>
              {/if}
              <Button
                size="sm"
                aria-label="{s.name} を編集"
                disabled={off !== null}
                onclick={() => open("sprint-edit", s)}
              >
                編集
              </Button>
              <Button
                size="sm"
                aria-label="{s.name} を削除"
                disabled={off !== null}
                onclick={() => open("sprint-rm", s)}
              >
                削除
              </Button>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="6" class="empty">
              {plans.loaded ? "スプリントはありません" : "読み込み中…"}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
</div>

<style>
  .page {
    box-sizing: border-box;
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    min-height: 0;
    padding-bottom: var(--space-28);
  }
  .msg {
    margin: var(--space-12) var(--gutter) 0;
    color: var(--danger);
  }
  .now {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-16);
    align-items: center;
    padding: var(--space-16) var(--gutter) 0;
  }
  .what {
    display: flex;
    flex: 1 1 280px;
    flex-direction: column;
    gap: var(--space-6);
    min-width: 0;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
    font-weight: 600;
  }
  .dates,
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .pick {
    display: inline-flex;
    align-self: flex-start;
    gap: 2px;
    padding: 3px;
    margin: 0;
    border: 0;
    border-radius: var(--radius-8);
    background: var(--sunken);
  }
  .pick button {
    min-height: 28px;
    padding: 0 var(--space-10);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    cursor: pointer;
  }
  .pick button[aria-pressed="true"] {
    background: var(--surface);
    font-weight: 600;
  }
  .stat {
    display: flex;
    flex-direction: column;
    min-width: 96px;
    padding: var(--space-10) var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-10);
  }
  .stat span {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .stat b {
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .board {
    flex: 0 0 auto;
    display: flex;
    flex-direction: column;
    height: 60vh;
    min-height: 320px;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-16) var(--gutter) 0;
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  th,
  td {
    padding: var(--space-6) var(--space-8);
    border-top: 1px solid var(--line);
    text-align: left;
  }
  thead th {
    border-top: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  tbody th {
    font-weight: 600;
  }
  .acts {
    display: flex;
    gap: var(--space-6);
    justify-content: flex-end;
  }
  .state {
    padding: 0 var(--space-6);
    border-radius: 999px;
    background: var(--sunken);
    font-size: var(--fs-11);
  }
  .state[data-state="active"] {
    background: var(--accent-weak);
  }
  .empty {
    color: var(--muted);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
