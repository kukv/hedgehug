<script lang="ts">
  import {
    diffText,
    type EstimateRow,
    emptyEstimates,
    hoursText,
    perPointText,
    pointsText,
    type Report,
    type ReportSelection,
    sprintScopeNote,
  } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";
  import { detail, links } from "../../wire";

  // 見積もりと実績(05「見積もりと実績」)。単位によらない。差が正の行は警告の色にする。
  interface Props {
    report: Report;
    storeId: string;
    storeName: string;
    onselect: (patch: Partial<ReportSelection>) => void;
  }

  let { report, storeId, storeName, onselect }: Props = $props();

  const options = $derived([
    { value: "", label: "すべて" },
    ...report.sprints.map((s) => ({ value: s.id, label: s.name })),
  ]);
  const total = $derived(report.estimateTotal);
  const totalDiff = $derived(diffText(total.diff));
  const route = (r: EstimateRow) => ({ name: "item" as const, store: storeName, item: r.display });
</script>

<section aria-label="見積もりと実績">
  <header>
    <h2>見積もりと実績</h2>
    <Select
      label="対象"
      {options}
      value={report.selection.estimate}
      onchange={(v) => onselect({ estimate: v })}
    />
  </header>
  {#if report.selection.estimate !== ""}
    <p class="note">{sprintScopeNote}</p>
  {/if}
  {#if report.estimates.length === 0}
    <p class="note">{emptyEstimates}</p>
  {:else}
    <table>
      <caption>
        見積もりと実績の表
      </caption>
      <thead>
        <tr>
          <th scope="col">アイテム</th>
          <th scope="col">ポイント</th>
          <th scope="col">見積もり時間</th>
          <th scope="col">作業時間</th>
          <th scope="col">差</th>
          <th scope="col">時間/pt</th>
        </tr>
      </thead>
      <tbody>
        {#each report.estimates as r (r.id)}
          {@const diff = diffText(r.diff)}
          <tr>
            <td>
              <code class="id">{r.display}</code>
              <a
                href={links.href(route(r))}
                onclick={(e) => links.click(e, route(r), () => void detail.open(storeId, r.id))}
                onauxclick={(e) => links.click(e, route(r))}
                >{r.title}</a
              >
            </td>
            <td>{pointsText(r.points)}</td>
            <td>{hoursText(r.estimate)}</td>
            <td>{hoursText(r.worked)}</td>
            <td class:over={diff.over}>{diff.text}</td>
            <td>{perPointText(r.perPoint)}</td>
          </tr>
        {/each}
        <tr class="total">
          <th scope="row">合計</th>
          <td>{pointsText(total.points)}</td>
          <td>{hoursText(total.estimate)}</td>
          <td>{hoursText(total.worked)}</td>
          <td class:over={totalDiff.over}>{totalDiff.text}</td>
          <td></td>
        </tr>
      </tbody>
    </table>
  {/if}
</section>

<style>
  section {
    margin-bottom: 28px;
  }
  header {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  h2 {
    font-size: 15px;
    margin: 0;
  }
  .note {
    margin: 8px 0 0;
    font-size: 12.5px;
    color: var(--muted);
  }
  table {
    border-collapse: collapse;
    margin-top: 8px;
    font-size: 12.5px;
  }
  caption {
    position: absolute;
    left: -9999px;
  }
  th,
  td {
    padding: 4px 12px;
    border-bottom: 1px solid var(--line);
    text-align: right;
  }
  th:first-child,
  td:first-child {
    text-align: left;
  }
  thead th {
    color: var(--muted);
    font-weight: 600;
  }
  .id {
    font-family: var(--font-mono);
    color: var(--muted);
    margin-right: 6px;
  }
  a {
    color: var(--ink);
    text-decoration: none;
  }
  .total th,
  .total td {
    font-weight: 600;
  }
  .over {
    color: var(--warning);
  }
</style>
