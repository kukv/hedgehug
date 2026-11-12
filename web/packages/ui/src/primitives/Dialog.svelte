<script lang="ts">
  import type { Snippet } from "svelte";
  import IconButton from "./IconButton.svelte";

  interface Props {
    open: boolean;
    title: string;
    wide?: boolean;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
  }

  let { open, title, wide = false, onclose, children, footer }: Props = $props();
  const titleId = $props.id();

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape" && !event.isComposing) onclose();
  }

  // ダイアログの外(暗くした背景)を押したときだけ閉じる。
  function onmousedown(event: MouseEvent) {
    if (event.target === event.currentTarget) onclose();
  }
</script>

<svelte:window {onkeydown} />

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="scrim" {onmousedown}>
    <div class="dialog" class:wide role="dialog" aria-modal="true" aria-labelledby={titleId}>
      <div class="dlg-head">
        <h2 id={titleId}>{title}</h2>
        <IconButton label="閉じる" size="sm" onclick={onclose}>×</IconButton>
      </div>
      <div class="dlg-body">{@render children()}</div>
      {#if footer}
        <div class="dlg-foot">{@render footer()}</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: var(--scrim);
    /* サイドバーの引き出し(50)から開くダイアログもあるので、その上に重ねる。メニュー(60)より下。 */
    z-index: 55;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: max(44px, env(safe-area-inset-top, 0px)) 16px 16px;
    overflow-y: auto;
  }
  .dialog {
    background: var(--surface);
    border-radius: 12px;
    box-shadow: var(--shadow-dialog);
    width: min(480px, 100%);
    display: flex;
    flex-direction: column;
    max-height: calc(100vh - 80px);
  }
  .wide {
    width: min(720px, 100%);
  }
  .dlg-head {
    display: flex;
    align-items: center;
    padding: 14px 16px 8px 20px;
  }
  .dlg-head h2 {
    margin: 0;
    font-size: 15px;
    flex: 1;
  }
  .dlg-body {
    padding: 4px 20px 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    overflow-y: auto;
  }
  .dlg-foot {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 12px 16px;
    border-top: 1px solid var(--line);
  }
</style>
