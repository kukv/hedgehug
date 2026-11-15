<script lang="ts">
  import { keyboardRows, treeKeyRows } from "@hedgehug/core";

  // 「キーボード」と「ツリー」の一覧(08「一般」)。01・04 の表の文のまま並べる。
</script>

<section aria-labelledby="keyboard-title">
  <h2 id="keyboard-title">キーボード</h2>
  <table aria-labelledby="keyboard-title">
    <thead>
      <tr>
        <th scope="col">キー</th>
        <th scope="col">動作</th>
      </tr>
    </thead>
    <tbody>
      {#each keyboardRows as row (row.key)}
        <tr>
          <td class="key">{row.key}</td>
          <td>{row.action}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>
<section aria-labelledby="tree-keys-title">
  <h2 id="tree-keys-title">ツリー</h2>
  <table aria-labelledby="tree-keys-title">
    <thead>
      <tr>
        <th scope="col">キー</th>
        <th scope="col">アイテムの行</th>
        <th scope="col">プロジェクトの行</th>
      </tr>
    </thead>
    <tbody>
      {#each treeKeyRows as row, i (i)}
        <tr>
          <td class="key">{row.key}</td>
          <td>{row.item}</td>
          <td>{row.project}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<style>
  section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h2 {
    margin: 0;
    font-size: 15px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12.5px;
  }
  th,
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    text-align: left;
    vertical-align: top;
  }
  th {
    color: var(--muted);
    font-weight: 600;
  }
  .key {
    white-space: nowrap;
  }
</style>
