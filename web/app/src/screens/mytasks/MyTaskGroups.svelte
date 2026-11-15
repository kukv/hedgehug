<script lang="ts">
  import { dayOf, groupMyTasks, myTasksEmpty } from "@hedgehug/core";
  import { StoreBadge } from "@hedgehug/ui";
  import { today } from "../../clock";
  import { myTaskBase, myTasks } from "../../myTasks";
  import { catalog } from "../../wire";
  import MyTaskRow from "./MyTaskRow.svelte";

  // マイタスクの本体(04「マイタスク」)。行をまとめ方の見出しに分けて出す。行がないときは、読み終えていて
  // 絞り込みなしの行も読めているときだけ空の文を出す(読み込み中や読めないときに「担当しているタスクはありません」と出さない)。
  const view = myTasks.state;
  const base = myTaskBase.rows;
  const stores = catalog.stores;
  const groups = $derived(groupMyTasks($view.rows, $view.groupBy, dayOf($today), $stores));
  const empty = $derived(
    $view.rows.length === 0 && $view.loaded && $base !== null ? myTasksEmpty($base.length) : null,
  );
</script>

<div class="content">
  {#if empty}
    <div class="empty">
      <p class="empty-title">{empty.title}</p>
      <p class="empty-note">{empty.body}</p>
    </div>
  {:else}
    {#each groups as g (g.key)}
      <section class="group">
        <h2 class="head" class:warn={g.warn}>
          {#if g.store}
            <StoreBadge name={g.store.name} color={g.store.color} />
          {/if}
          {g.label}
          <span class="count">{g.rows.length}</span>
        </h2>
        <ul class="rows">
          {#each g.rows as row (`${row.storeId}/${row.itemId}`)}
            <MyTaskRow {row} />
          {/each}
        </ul>
      </section>
    {/each}
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 0 24px 40px;
    min-height: 0;
  }
  .empty {
    padding: 40px 0;
    text-align: center;
  }
  .empty-title {
    margin: 0;
    font-weight: 500;
  }
  .empty-note {
    margin: 6px auto 12px;
    max-width: 520px;
    color: var(--muted);
    font-size: 12.5px;
  }
  .group {
    margin-top: 16px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 6px;
    font-size: 13px;
    font-weight: 600;
  }
  .warn {
    color: var(--danger);
  }
  .count {
    color: var(--muted);
    font-weight: 400;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  @container frame (max-width: 860px) {
    .content {
      padding-left: max(16px, env(safe-area-inset-left, 0px));
      padding-right: max(16px, env(safe-area-inset-right, 0px));
    }
  }
</style>
