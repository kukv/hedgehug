<script lang="ts">
  import { hasRules, type ProjectView, projectWorkflowRows, rulesOf } from "@hedgehug/core";
  import { projectsPort, rulesEditor } from "../../projectEditor";
  import WorkflowTable from "../workspace-settings/WorkflowTable.svelte";
  import ResetRulesButton from "./ResetRulesButton.svelte";

  // 「ワークフローの制約」の区画(03「プロジェクトの遷移の制約」)。表は WorkflowTable。チェックのたびに保存する。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const rows = $derived(projectWorkflowRows(view.settings, view.project.id));
  const empty = $derived(!hasRules(rulesOf(view.settings, view.project.id), "workflow"));

  // 断られたとき、チェックを保存済みの値に戻すため、操作のたびに表を作り直す(値が変わらないと DOM が戻らない)。
  let round = $state(0);
  async function change(from: string, to: string, allowed: boolean) {
    const p = view.project;
    await rulesEditor.run(() => projectsPort.setTransition(p.storeId, p.id, from, to, allowed));
    round += 1;
  }
</script>

<section class="sec">
  <h2 id="rules-workflow" tabindex="-1">ワークフローの制約</h2>
  <p class="help">ワークスペースの制約に追加することだけできます(緩めることはできません)</p>
  {#key round}
    <WorkflowTable
      {rows}
      readOnly={view.store.readOnly}
      onchange={(f, t, c) => void change(f, t, c)}
    />
  {/key}
  <ResetRulesButton {view} section="workflow" label="ワークフローの制約" {empty} />
</section>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h2 {
    font-size: 14px;
    margin: 0;
  }
  .help {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
