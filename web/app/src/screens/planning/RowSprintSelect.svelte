<script lang="ts">
  import {
    asOptions,
    type BacklogRow,
    chosen,
    type PlanningOverview,
    readOnlyNote,
    sprintChoices,
  } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";
  import { backlog } from "../../planning";

  // バックログの画面の行の「スプリント」の選択(05「行」、F2a-8)。BacklogRow が 200 行なので分ける。
  interface Props {
    row: BacklogRow;
    // sprint は、行のある枠のスプリントの ID(バックログは空)。
    sprint: string;
    overview: PlanningOverview | null;
    readOnly: boolean;
  }

  let { row, sprint, overview, readOnly }: Props = $props();

  // スプリントの選択肢は、「バックログ」と完了していないスプリント(概要を読むまでは出さない)。
  const choices = $derived(overview ? sprintChoices(overview, sprint) : []);
  const current = $derived(chosen(choices, sprint));
  let selected = $state("");
  $effect(() => {
    selected = current;
  });
  // 保存のあとは、いまの値に戻す(断られて読み直しても値が変わらないと、上の $effect は走り直さない)。
  async function pickSprint(id: string) {
    await backlog.setSprint(row, id);
    selected = current;
  }
</script>

<Select
  label="スプリント"
  size="sm"
  disabled={readOnly || !overview}
  title={readOnly ? readOnlyNote : undefined}
  options={asOptions(choices)}
  bind:value={selected}
  onchange={(id) => void pickSprint(id)}
/>
