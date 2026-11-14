<script lang="ts">
  import type { SearchRow as Row } from "@hedgehug/core";
  import SearchRow from "./SearchRow.svelte";

  // 検索のテーブル(04「テーブル」、E1-18)。初めの 7 列を読むだけで出す。列の操作・選択・グループは E2。
  interface Props {
    rows: Row[];
  }

  let { rows }: Props = $props();
  // 終了日の「期限切れ」「今日」は、画面を開いたときの今日で決める(ツリーと同じ)。
  const today = new Date();
  const columns = ["ID", "タイトル", "ステータス", "担当者", "優先度", "終了日", "ポイント"];
</script>

<table class="table">
  <thead>
    <tr>
      {#each columns as column (column)}
        <th scope="col">{column}</th>
      {/each}
    </tr>
  </thead>
  <tbody>
    <!-- 同じ ID が別の Store にもあるので、Store とアイテムの内部 ID で見分ける -->
    {#each rows as row (`${row.storeId}/${row.itemId}`)}
      <SearchRow {row} {today} />
    {/each}
  </tbody>
</table>

<style>
  .table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  th {
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--surface);
    color: var(--muted);
    font-size: 12px;
    font-weight: 600;
    text-align: left;
    white-space: nowrap;
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
  }
</style>
