<script lang="ts">
  import {
    type Item,
    itemDisplayId,
    type ProjectView,
    type Route,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, KindIcon } from "@hedgehug/ui";
  import { descriptionSaves, detail, links } from "../../wire";
  import CommentsSection from "../notes/CommentsSection.svelte";
  import DescriptionSection from "../notes/DescriptionSection.svelte";
  import HistorySection from "../notes/HistorySection.svelte";
  import OverviewProps from "./OverviewProps.svelte";
  import OverviewRepos from "./OverviewRepos.svelte";

  // プロジェクトの「概要」のタブ(02「概要の編集」)。左の列に説明・Epic・リポジトリ・コメント・履歴、右の列に項目の編集。
  // 「PR と Issue」の区画は D3・D4 が、Epic の期限・進捗・担当者の列は別のトラックが足す。
  interface Props {
    view: ProjectView;
    onaddepic: () => void;
  }

  let { view, onaddepic }: Props = $props();

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
          <div class="list-row">
            <KindIcon kind="Epic" />
            <code class="id">{id(e)}</code>
            <a
              class="row-title"
              href={links.href(route(e))}
              onclick={(ev) =>
                links.click(ev, route(e), () => void detail.open(view.store.id, e.id))}
              onauxclick={(ev) => links.click(ev, route(e))}
              >{e.title}</a
            >
          </div>
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
  .list-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1;
    min-width: 0;
  }
  .empty {
    color: var(--muted);
    margin: 0;
  }
</style>
