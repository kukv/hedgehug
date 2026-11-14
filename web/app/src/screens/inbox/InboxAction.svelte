<script lang="ts">
  import type { InboxRow } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { inbox } from "../../inbox";

  // 自動化が制約で止まった通知の、止まった理由と 2 つのボタン(06「操作」)。ボタンの状態はサーバーが決める
  // (C4a-10)。押したあとは、ボタンの代わりに結果のピルを出す。
  interface Props {
    row: InboxRow;
  }

  let { row }: Props = $props();

  const a = $derived(row.action);
  const results = { applied: "適用しました", kept: "このままにしました" } as const;
</script>

<div class="action">
  {#if row.reason}
    <p class="reason">{row.reason}</p>
  {/if}
  {#if a?.result}
    <span class="result">{results[a.result]}</span>
  {:else if a?.visible}
    <div class="buttons">
      <Button size="sm" disabled={!a.applyEnabled} onclick={() => void inbox.apply(row)}>
        制約を無視して {a.statusName} にする
      </Button>
      <Button size="sm" disabled={!a.keepEnabled} onclick={() => void inbox.keep(row)}>
        変更しない
      </Button>
      {#if a.note}
        <span class="note">{a.note}</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .action {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .reason {
    margin: 0;
    color: var(--muted);
    font-size: 12.5px;
  }
  .buttons {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .note {
    color: var(--muted);
    font-size: 12px;
  }
  .result {
    align-self: flex-start;
    font-size: 11px;
    padding: 1px 7px;
    border-radius: 999px;
    background: var(--sunken);
    color: var(--muted);
  }
</style>
