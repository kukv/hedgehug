<script lang="ts">
  import type { StructureRow } from "@hedgehug/core";
  import { KindIcon, type KindName } from "@hedgehug/ui";

  // 作成ダイアログの「一緒に作るもの」(02「ダイアログの項目」)。読み取り専用のツリー。
  interface Props {
    structure: { heading: string; rows: StructureRow[] };
  }

  let { structure }: Props = $props();
</script>

<section class="structure" aria-label={structure.heading}>
  <h3>{structure.heading}</h3>
  <ul>
    {#each structure.rows as r, i (i)}
      <li style:padding-left="{r.depth * 20}px">
        <KindIcon kind={r.kind as KindName} />
        <span class="title">{r.title}</span>
        <span class="assignee">{r.assignee}</span>
      </li>
    {/each}
  </ul>
</section>

<style>
  .structure h3 {
    font-size: 12px;
    margin: 0 0 4px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .title {
    flex: 1;
  }
  .assignee {
    color: var(--muted);
  }
</style>
