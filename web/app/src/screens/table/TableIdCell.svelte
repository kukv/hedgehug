<script lang="ts">
  import type { SearchRow } from "@hedgehug/core";

  // テーブルの ID の列のセル(04「テーブル」、E2a-12)。Store の色の点・いまの ID・旧 ID だけで一致した印を出す。
  interface Props {
    row: SearchRow;
  }

  let { row }: Props = $props();
</script>

<td>
  <span class="with-icon">
    <i
      class="sdot"
      style:background="var(--t-{row.storeColor}-fg)"
      role="img"
      aria-label={row.storeName}
      title={row.storeName}
    ></i>
    <code class="id">{row.display}</code>
    {#if row.oldIdHit}
      <span class="old">旧 ID</span>
    {/if}
  </span>
</td>

<style>
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    white-space: nowrap;
  }
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .old {
    font-size: 11px;
    color: var(--muted);
  }
  .with-icon {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
