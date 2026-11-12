<script lang="ts">
  import {
    formatDay,
    itemDisplayId,
    ownerName,
    type ProjectView,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, KindIcon } from "@hedgehug/ui";

  // プロジェクトの「概要」のタブ。A2 では「Epic」の区画と、プロパティの表示だけ(A2 の計画の決定 16)。
  interface Props {
    view: ProjectView;
    onaddepic: () => void;
  }

  let { view, onaddepic }: Props = $props();

  const year = new Date().getFullYear();
  const p = $derived(view.project);
</script>

<div class="overview-frame">
  <div class="overview">
    <section class="card-sec" aria-label="Epic">
      <div class="sec-head">
        <h2>Epic</h2>
        <span class="grow"></span>
        <Button
          size="sm"
          disabled={view.store.readOnly}
          title={view.store.readOnly ? readOnlyNote : undefined}
          onclick={onaddepic}
          ><span aria-hidden="true">+</span>
          Epic を追加</Button
        >
      </div>
      {#each view.epics as e (e.id)}
        <div class="list-row">
          <KindIcon kind="Epic" />
          <code class="id">{itemDisplayId(view.workspace.prefix, e.number)}</code>
          <span class="row-title">{e.title}</span>
        </div>
      {/each}
    </section>
    <section class="props" aria-label="プロパティ">
      <div class="prop"><span class="prop-k">ステータス</span><span>{p.status}</span></div>
      <div class="prop">
        <span class="prop-k">オーナー</span><span>{ownerName(p.owner, view.members)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">開始日</span><span>{formatDay(p.start, year)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">終了日</span><span>{formatDay(p.end, year)}</span>
      </div>
    </section>
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
  }
  @container overview (max-width: 640px) {
    .overview {
      grid-template-columns: minmax(0, 1fr);
    }
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
    flex: 1;
    min-width: 0;
  }
  .props {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .prop {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr);
    align-items: center;
  }
  .prop-k {
    color: var(--muted);
    font-size: 12.5px;
  }
</style>
