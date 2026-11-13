<script lang="ts">
  import {
    categoryLabels,
    hasRules,
    type ProjectView,
    readOnlyNote,
    rulesOf,
    usedStatusRows,
  } from "@hedgehug/core";
  import { Checkbox } from "@hedgehug/ui";
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

  // 断られたとき、チェックを保存済みの値に戻すため、操作のたびに行を作り直す(値が変わらないと DOM が戻らない)。
  let round = $state(0);
  async function change(statusId: string, used: boolean) {
    const p = view.project;
    await rulesEditor.run(() => projectsPort.setStatusUsed(p.storeId, p.id, statusId, used));
    round += 1;
  }
</script>

<section class="sec">
  <h2 id="rules-statuses" tabindex="-1">使うステータス <span class="count">{data.count}</span></h2>
  <p class="help">
    チェックを外したステータスは、このプロジェクトのステータスのメニュー・作成ダイアログ・カンバンの列に出ません。外しても、そのステータスのアイテムは変わりません。
  </p>
  <ul class="rows">
    {#key round}
      {#each data.rows as row (row.id)}
        <li>
          <span class="box">
            <Checkbox
              label={row.name}
              disabled={disabled || row.disabled}
              title={disabled ? readOnlyNote : row.note}
              bind:checked={() => row.checked, (v) => void change(row.id, v)}
            />
          </span>
          <i class="dot" style:background="var(--c-{row.color}-dot)"></i>
          <span class="name">{row.name}</span>
          <span class="cat">{categoryLabels[row.category]}</span>
          {#if row.note}
            <span class="note">{row.note}</span>
          {/if}
          {#if row.usage}
            <span class="note">{row.usage}</span>
          {/if}
        </li>
      {/each}
    {/key}
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
  /* チェックボックスの文字は読み上げ用なので、画面では隠す。 */
  .box :global(span) {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex: none;
  }
  .cat,
  .note {
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
