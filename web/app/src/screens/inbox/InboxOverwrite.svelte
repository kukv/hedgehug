<script lang="ts">
  import type { InboxRow } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { inbox } from "../../inbox";

  // 上書きの通知の、「あなたの値 → 残った値」と「自分の値に戻す」(06「操作」)。ボタンの状態はサーバーが決める
  // (C4c2-11)。押したあとは、ボタンの代わりに結果のピルを出す。
  interface Props {
    row: InboxRow;
  }

  let { row }: Props = $props();

  const o = $derived(row.overwrite);
</script>

{#if o}
  <div class="overwrite">
    <p class="values">あなたの値 {o.mine} → 残った値 {o.kept}</p>
    {#if o.result}
      <span class="result">自分の値に戻しました</span>
    {:else if o.visible}
      <div class="buttons">
        <Button size="sm" disabled={!o.enabled} onclick={() => void inbox.revert(row)}>
          自分の値に戻す
        </Button>
        {#if o.note}
          <span class="note">{o.note}</span>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .overwrite {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .values {
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
