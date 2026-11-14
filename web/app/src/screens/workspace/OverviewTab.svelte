<script lang="ts">
  import { activeReports, reloadActiveOnDayChange } from "../../reports";
  import SprintPointsCard from "./SprintPointsCard.svelte";

  // ワークスペースの「概要」のタブ。統計のカードの並び(04「統計」)の場所で、F3 は完了ポイントだけを置く。
  // ほかのカードとプロジェクトの一覧は、04 を持つトラック E がこの並びに足す(F3-18)。
  interface Props {
    storeId: string;
    workspaceId: string;
  }

  let { storeId, workspaceId }: Props = $props();

  // タブを開いているあいだだけ読む。Store の出来事で読み直す(reports.ts)。
  const sid = $derived(storeId);
  const wid = $derived(workspaceId);
  $effect(() => {
    void activeReports.open(sid, wid);
    return () => activeReports.close();
  });
  const loaded = activeReports.state;
  const view = $derived(
    $loaded && $loaded.storeId === sid && $loaded.workspaceId === wid ? $loaded : null,
  );

  $effect(reloadActiveOnDayChange);
</script>

<div class="stats">
  {#if view}
    <SprintPointsCard active={view.active} />
  {/if}
</div>

<style>
  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 12px;
  }
</style>
