<script lang="ts">
  import { activityDays, activityEmpty, dayOf } from "@hedgehug/core";
  import { Button, PageHead } from "@hedgehug/ui";
  import { onMount } from "svelte";
  import { activity } from "../../activity";
  import { syncNow } from "../../wire";
  import ActivityFilters from "./ActivityFilters.svelte";
  import ActivityRow from "./ActivityRow.svelte";

  // アクティビティの画面(06「アクティビティ」)。日付ごとにまとめ、「今日」「昨日」は syncNow の日付で決める
  // (日付が変わったら見出しも変わる。C4d-15)。
  interface Props {
    ondrawer: () => void;
    alert: boolean;
  }

  let { ondrawer, alert }: Props = $props();

  const view = activity.state;
  const days = $derived(activityDays($view.rows, dayOf($syncNow)));

  onMount(() => void activity.open());
</script>

<PageHead
  title="アクティビティ"
  note="Store の git 履歴。1 つの変更が 1 つのコミットです"
  {ondrawer}
  {alert}
/>
<ActivityFilters />
<div class="content">
  {#if $view.rows.length > 0}
    {#each days as day (day.head)}
      <section aria-label={day.head}>
        <h2 class="day">{day.head}</h2>
        <ul class="rows">
          {#each day.rows as row (row.key)}
            <ActivityRow {row} />
          {/each}
        </ul>
      </section>
    {/each}
    {#if $view.more}
      <div class="more">
        <Button size="sm" disabled={$view.loading} onclick={() => void activity.more()}>
          さらに読み込む
        </Button>
      </div>
    {/if}
  {:else if $view.loaded && !$view.loading}
    <div class="empty">
      <p class="empty-title">{activityEmpty}</p>
    </div>
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 8px 24px 40px;
    min-height: 0;
  }
  .day {
    margin: 16px 0 4px;
    font-size: 12.5px;
    font-weight: 600;
    color: var(--muted);
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .more {
    display: flex;
    justify-content: center;
    padding: 12px 0;
  }
  .empty {
    padding: 40px 0;
    text-align: center;
  }
  .empty-title {
    margin: 0;
    font-weight: 500;
  }
  @container frame (max-width: 860px) {
    .content {
      padding: 8px max(16px, env(safe-area-inset-right, 0px)) 32px
        max(16px, env(safe-area-inset-left, 0px));
    }
  }
</style>
