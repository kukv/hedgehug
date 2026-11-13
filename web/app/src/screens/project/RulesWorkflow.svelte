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

  function change(from: string, to: string, allowed: boolean) {
    const p = view.project;
    void rulesEditor.run(() => projectsPort.setTransition(p.storeId, p.id, from, to, allowed));
  }
</script>

<section class="sec">
  <h2 id="rules-workflow" tabindex="-1">ワークフローの制約</h2>
  <p class="help">ワークスペースの制約に追加することだけできます(緩めることはできません)</p>
  <WorkflowTable {rows} readOnly={view.store.readOnly} onchange={change} />
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
