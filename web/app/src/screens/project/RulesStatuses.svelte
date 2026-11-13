<script lang="ts">
  import {
    categoryLabels,
    hasRules,
    type ProjectView,
    readOnlyNote,
    rulesOf,
    usedStatusRows,
  } from "@hedgehug/core";
  import { Checkbox, StatusDot } from "@hedgehug/ui";
  import { projectsPort, rulesEditor } from "../../projectEditor";
  import ResetRulesButton from "./ResetRulesButton.svelte";

  // 「使うステータス」の区画(03「使うステータス」)。チェックのたびに保存する。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const disabled = $derived(view.store.readOnly);
  const data = $derived(usedStatusRows(view.settings, view.project.id, view.items));
  const empty = $derived(!hasRules(rulesOf(view.settings, view.project.id), "statuses"));

  function change(statusId: string, used: boolean) {
    const p = view.project;
    void rulesEditor.run(() => projectsPort.setStatusUsed(p.storeId, p.id, statusId, used));
  }
</script>

<section class="sec">
  <h2 id="rules-statuses" tabindex="-1">使うステータス <span class="count">{data.count}</span></h2>
  <p class="help">
    チェックを外したステータスは、このプロジェクトのステータスのメニュー・作成ダイアログ・カンバンの列に出ません。外しても、そのステータスのアイテムは変わりません。
  </p>
  <ul class="rows">
    {#each data.rows as row (row.id)}
      <li>
        <Checkbox
          label={row.name}
          disabled={disabled || row.disabled}
          title={disabled ? readOnlyNote : row.note}
          bind:checked={() => row.checked, (v) => change(row.id, v)}
        />
        <StatusDot category={row.category} label={row.name} />
        <span class="cat">{categoryLabels[row.category]}</span>
        {#if row.note}
          <span class="note">{row.note}</span>
        {/if}
        {#if row.usage}
          <span class="note">{row.usage}</span>
        {/if}
      </li>
    {/each}
  </ul>
  <ResetRulesButton {view} section="statuses" label="使うステータス" {empty} />
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
  .count {
    font-weight: 400;
    color: var(--muted);
  }
  .help {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  li {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .cat,
  .note {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
