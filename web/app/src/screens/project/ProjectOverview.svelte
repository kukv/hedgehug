<script lang="ts">
  import {
    type Item,
    itemDisplayId,
    type ProjectView,
    type Route,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { descriptionSaves, detail, links, projectScreen } from "../../wire";
  import CommentsSection from "../notes/CommentsSection.svelte";
  import DescriptionSection from "../notes/DescriptionSection.svelte";
  import HistorySection from "../notes/HistorySection.svelte";
  import EpicRow from "./EpicRow.svelte";
  import OverviewProps from "./OverviewProps.svelte";
  import OverviewRepos from "./OverviewRepos.svelte";

  // プロジェクトの「概要」のタブ(02「概要の編集」)。左の列に説明・Epic・リポジトリ・コメント・履歴、右の列に項目の編集。
  // 「PR と Issue」の区画は OverviewRepos.svelte の下段。Epic の行(期限・進捗・ステータス・担当者)は EpicRow.svelte。
  interface Props {
    view: ProjectView;
    onaddepic: () => void;
  }

  let { view, onaddepic }: Props = $props();

  // 期限の判定に使う今日は、画面を出したときの日付(ツリーと同じ)。
  const today = new Date();
  const p = $derived(view.project);
  const readonly = $derived(view.store.readOnly);
  const target = $derived({ kind: "project" as const, storeId: view.store.id, id: p.id });
  // 項目を変えて保存したら、履歴を読み直す(HistorySection の refresh に渡す)。
  const historyKey = $derived([p.title, p.status, p.owner, p.start, p.end].join("\n"));
  const id = (it: Item) => itemDisplayId(view.workspace.prefix, it.number);
  const route = (it: Item): Route => ({ name: "item", store: view.store.name, item: id(it) });
</script>

<div class="overview-frame">
  <div class="overview">
    <div class="main">
      <DescriptionSection {target} {readonly} />
      <section class="card-sec" aria-label="Epic">
        <div class="sec-head">
          <h2>Epic</h2>
          <span class="grow"></span>
          <Button
            size="sm"
            disabled={readonly}
            title={readonly ? readOnlyNote : undefined}
            onclick={onaddepic}
            ><span aria-hidden="true">+</span>
            Epic を追加</Button
          >
        </div>
        {#each view.epics as e (e.id)}
          <EpicRow
            epic={e}
            displayId={id(e)}
            href={links.href(route(e))}
            items={view.items}
            settings={view.settings}
            members={view.members}
            {today}
            readOnly={readonly}
            onopen={(ev) => links.click(ev, route(e), () => void detail.open(view.store.id, e.id))}
            onauxopen={(ev) => links.click(ev, route(e))}
            onstatus={(statusId) => void projectScreen.setStatus(e.id, statusId)}
          />
        {:else}
          <p class="empty">Epic はまだありません。Epic の下に Story・Task・Bug を作れます。</p>
        {/each}
      </section>
      <OverviewRepos {view} />
      <CommentsSection {target} {readonly} />
      <!-- 履歴は、項目の変更と説明の保存のあとに読み直す -->
      <HistorySection {target} refresh={[historyKey, $descriptionSaves]} />
    </div>
    <OverviewProps {view} />
  </div>
</div>

<style>
  .overview-frame {
    container: overview / inline-size;
  }
  .overview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 260px;
    gap: 24px;
    align-items: start;
  }
  @container overview (max-width: 640px) {
    .overview {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .main {
    display: flex;
    flex-direction: column;
    gap: 24px;
    min-width: 0;
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .empty {
    color: var(--muted);
    margin: 0;
  }
</style>
