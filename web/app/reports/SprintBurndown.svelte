<script lang="ts">
  import { errorMessage } from "../../core/messages";
  import type { Sprint } from "../../core/planning";
  import { amountLabel, type BurndownView, burndownFacts } from "../../core/reports";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import BurndownPlot from "./BurndownPlot.svelte";
  import { useReport } from "./report.svelte";

  // The top of the board of a running sprint: what is left, what joined, and the burndown.
  type Props = { entry: WorkspaceEntry; sprint: Sprint };
  let { entry, sprint }: Props = $props();

  const c = core();
  const report = useReport<BurndownView>(
    () => entry.store,
    () => c.api.reportBurndown({ store: entry.store, workspace: entry.name, sprint: sprint.name }),
  );
  const view = $derived(report.value);
  const facts = $derived(view ? burndownFacts(view) : null);
</script>

{#if report.error}
  <p class="msg" role="alert">{errorMessage(report.error)}</p>
{:else if view && facts}
  <div class="stat">
    <span>残り {view.by === "count" ? "件数" : "ポイント"}</span>
    <b
      >{amountLabel(facts.remaining, view.by)}
      <small>/ {amountLabel(facts.scope, view.by)}</small></b
    >
  </div>
  <div class="stat">
    <span>途中で追加</span>
    <b>+{amountLabel(facts.added, view.by)}</b>
  </div>
  <div class="figure">
    <BurndownPlot {view} height={120} />
  </div>
{/if}

<style>
  .stat {
    display: flex;
    flex-direction: column;
    justify-content: center;
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
  .stat small {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 400;
  }
  .figure {
    flex: 0 1 380px;
    min-width: 240px;
    padding: var(--space-10) var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-10);
  }
  .msg {
    margin: 0;
    color: var(--danger);
  }
</style>
