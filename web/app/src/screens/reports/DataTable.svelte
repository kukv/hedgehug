<script lang="ts">
  // 「表で見る」の折りたたみと表。開閉は保存しない(初めは閉じる)。
  interface Props {
    caption: string;
    columns: string[];
    rows: string[][];
  }

  let { caption, columns, rows }: Props = $props();
</script>

<details class="data">
  <summary>表で見る</summary>
  <table>
    <caption>
      {caption}
    </caption>
    <thead>
      <tr>
        {#each columns as column (column)}
          <th scope="col">{column}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as row, i (i)}
        <tr>
          {#each row as cell, j (j)}
            <td>{cell}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</details>

<style>
  .data {
    margin-top: 8px;
    font-size: 12.5px;
  }
  summary {
    cursor: pointer;
    color: var(--accent);
  }
  table {
    border-collapse: collapse;
    margin-top: 8px;
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
  th {
    color: var(--muted);
    font-weight: 600;
  }
</style>
