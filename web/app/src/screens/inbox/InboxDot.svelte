<script lang="ts">
  import type { InboxRow } from "@hedgehug/core";
  import { inbox } from "../../inbox";

  // 受信箱の行の左端の点。押すと未読と既読を切り替える。
  let { row }: { row: InboxRow } = $props();
</script>

<button
  type="button"
  class="dot"
  class:unread={!row.read}
  aria-label={row.read ? "未読にする" : "既読にする"}
  title={row.read ? "未読にする" : "既読にする"}
  onclick={() => void inbox.toggle(row)}
>
  {#if !row.read}
    <span class="sr-only">未読</span>
  {/if}
</button>

<style>
  .dot {
    flex: none;
    width: 24px;
    height: 24px;
    margin: -2px -3px -2px -3px;
    border: 0;
    border-radius: 50%;
    background: none;
    cursor: pointer;
    position: relative;
  }
  .dot::after {
    content: "";
    position: absolute;
    inset: 8px;
    border-radius: 50%;
    border: 1px solid var(--line);
  }
  .unread::after {
    background: var(--accent);
    border-color: var(--accent);
  }
  .dot:hover {
    background: var(--hover);
  }
  .dot:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
</style>
