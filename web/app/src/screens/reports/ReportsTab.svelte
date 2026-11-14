<script lang="ts">
  import { dayOf, type ReportSelection } from "@hedgehug/core";
  import { reports } from "../../reports";
  import { syncNow, toasts } from "../../wire";
  import BurndownSection from "./BurndownSection.svelte";
  import CycleSection from "./CycleSection.svelte";
  import EstimatesSection from "./EstimatesSection.svelte";
  import FlowSection from "./FlowSection.svelte";
  import ReportHead from "./ReportHead.svelte";
  import VelocitySection from "./VelocitySection.svelte";

  // ワークスペースの「レポート」のタブ(05「レポート」)。選択は Store に書かないので、閲覧のみの Store でも変えられる(F3-7)。
  interface Props {
    storeId: string;
    storeName: string;
    workspaceId: string;
  }

  let { storeId, storeName, workspaceId }: Props = $props();

  // タブを開いているあいだだけ読む。Store の出来事で読み直す(reports.ts)。
  // 親が Store の読み直しで作り直した値を渡しても、ID が同じなら開き直さない(開き直すと「表で見る」が閉じる)。
  const sid = $derived(storeId);
  const wid = $derived(workspaceId);
  $effect(() => {
    void reports.open(sid, wid);
    return () => reports.close();
  });
  const loaded = reports.state;
  const view = $derived(
    $loaded && $loaded.storeId === sid && $loaded.workspaceId === wid ? $loaded : null,
  );

  // 日付が変わったら読み直す(01「日付と時刻」)。開いたときの 1 回とは別に数える。
  const today = $derived(dayOf($syncNow));
  const thisYear = $derived($syncNow.getFullYear());
  let seen = dayOf($syncNow);
  $effect(() => {
    if (today === seen) return;
    seen = today;
    void reports.reload();
  });

  async function choose(patch: Partial<ReportSelection>) {
    try {
      await reports.select(patch);
    } catch {
      toasts.show("保存できませんでした", { error: true });
    }
  }
</script>

{#if view}
  <ReportHead report={view.report} onselect={choose} />
  <BurndownSection report={view.report} {today} {thisYear} onselect={choose} />
  <VelocitySection report={view.report} />
  <FlowSection
    report={view.report}
    period={view.period}
    {today}
    {thisYear}
    onlayers={(layers) => void choose({ layers })}
  />
  <CycleSection report={view.report} />
  <EstimatesSection report={view.report} {storeId} {storeName} onselect={choose} />
{/if}
