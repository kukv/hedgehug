<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    open: boolean;
    // full は、全画面(サイドバーの右いっぱい。860px 以下の枠では画面いっぱい)にする。
    full?: boolean;
    // label は、パネルの名前(読み上げと、テストでの指し方)。
    label: string;
    // onclose は、Esc を押したときに呼ぶ。ダイアログ(aria-modal)が開いているあいだは、ダイアログが Esc を受ける。
    onclose: () => void;
    head: Snippet;
    children: Snippet;
  }

  let { open, full = false, label, onclose, head, children }: Props = $props();

  function onkeydown(event: KeyboardEvent) {
    // 日本語入力の変換中の Esc は、変換の取り消しに使う。パネルは閉じない。
    if (!open || event.key !== "Escape" || event.isComposing) return;
    if (document.querySelector('[aria-modal="true"]')) return;
    // 値を選ぶメニューが開いているあいだは、メニューが Esc を受ける。
    if (document.querySelector('[role="menu"]')) return;
    onclose();
  }
</script>

<svelte:window {onkeydown} />

<!-- AppFrame の中に置く(860px 以下の判定に、AppFrame の枠のコンテナクエリを使う)。 -->
{#if open}
  <section class="panel" class:full aria-label={label}>
    <header class="panel-head">{@render head()}</header>
    <div class="panel-body">{@render children()}</div>
  </section>
{/if}

<style>
  .panel {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    width: min(880px, 100vw);
    background: var(--surface);
    border-left: 1px solid var(--line);
    box-shadow: var(--shadow-dialog);
    z-index: 31;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding-top: env(safe-area-inset-top, 0px);
    padding-bottom: env(safe-area-inset-bottom, 0px);
    padding-right: env(safe-area-inset-right, 0px);
  }
  .panel-head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 10px 14px;
    border-bottom: 1px solid var(--line);
    position: sticky;
    top: 0;
    background: var(--surface);
    z-index: 2;
  }
  .panel-body {
    padding: 14px 20px 40px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .full {
    left: var(--sidebar-w);
    width: auto;
    box-shadow: none;
  }
  .full .panel-body {
    width: 100%;
    max-width: 1240px;
    margin: 0 auto;
    padding: 20px 32px 48px;
  }
  @container frame (max-width: 860px) {
    .full {
      left: 0;
    }
    .full .panel-body {
      padding: 14px 16px 40px;
    }
  }
</style>
