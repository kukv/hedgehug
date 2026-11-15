<script lang="ts">
  import { type ActivityKind, activityKindOptions, workspaceOptions } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";
  import { activity } from "../../activity";
  import { catalog } from "../../wire";

  // アクティビティの絞り込み(06「絞り込み」)。画面の状態で、アドレスにも Store にも入れない(C4d-15)。
  // ワークスペースの選択肢は、サイドバーの Store の一覧から作る(C4d-10)。前に開いたときの絞り込みを出す。
  const view = activity.state;
  const stores = catalog.stores;
  const workspaces = $derived(workspaceOptions($stores));
  // 選んでいたワークスペースが一覧からなくなったら(削除・別の Store へ移動・Store の非表示)、「すべて」に戻す。
  $effect(() => {
    const w = $view.filter.workspace;
    if (w && !workspaces.some((o) => o.value === w)) void activity.setFilter({ workspace: "" });
  });
</script>

<div class="filters">
  <span class="label" aria-hidden="true">ワークスペース</span>
  <Select
    label="ワークスペース"
    size="sm"
    options={workspaces}
    bind:value={() => $view.filter.workspace, (v) => void activity.setFilter({ workspace: v })}
  />
  <span class="label" aria-hidden="true">種類</span>
  <Select
    label="種類"
    size="sm"
    options={activityKindOptions}
    bind:value={
      () => $view.filter.kind,
      (v) => void activity.setFilter({ kind: v as ActivityKind })
    }
  />
</div>

<style>
  .filters {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding: 8px 24px;
    border-bottom: 1px solid var(--line);
  }
  .label {
    color: var(--muted);
    font-size: 12.5px;
  }
  .label + :global(.select) {
    margin-right: 8px;
  }
  @container frame (max-width: 860px) {
    .filters {
      padding: 8px max(16px, env(safe-area-inset-right, 0px)) 8px
        max(16px, env(safe-area-inset-left, 0px));
    }
  }
</style>
