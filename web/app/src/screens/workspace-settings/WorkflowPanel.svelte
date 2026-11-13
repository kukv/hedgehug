<script lang="ts">
  import { type SettingsState, workflowHelp, workflowRows } from "@hedgehug/core";
  import { workspaceSettings } from "../../wire";
  import WorkflowTable from "./WorkflowTable.svelte";

  // ワークフローのタブ(03「遷移の制約」)。表は WorkflowTable。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  const rows = $derived(workflowRows(s.detail.settings));

  function change(from: string, to: string, checked: boolean) {
    void workspaceSettings.run((p, st, w) => p.setTransition(st, w, from, to, checked));
  }
</script>

<div class="panel">
  <p class="help">{workflowHelp}</p>
  <WorkflowTable {rows} readOnly={s.readOnly} onchange={change} />
</div>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .help {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
