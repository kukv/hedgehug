<script lang="ts">
  import Icon from "../icons/Icon.svelte";
  import { onLinkClick } from "../router";
  import { dismissToast, toasts, undoToast } from "../toast.svelte";
</script>

<div class="stack" role="status" aria-live="polite">
  {#each toasts.list as t (t.id)}
    {@const link = t.link}
    <div class="toast">
      <span class="msg">{t.message}</span>
      {#if link}
        <a class="act" href={link.href} onclick={(e) => onLinkClick(e, link.href)}>{link.label}</a>
      {/if}
      {#if t.undo}
        <button type="button" class="act" onclick={() => undoToast(t.id)}>取り消し</button>
      {/if}
      <button type="button" class="x" aria-label="閉じる" onclick={() => dismissToast(t.id)}>
        <Icon name="close" size={14} />
      </button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed;
    left: var(--space-16);
    bottom: var(--space-16);
    z-index: 200;
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    max-width: calc(100vw - 2 * var(--space-16));
  }
  .toast {
    display: flex;
    align-items: center;
    gap: var(--space-12);
    padding: var(--space-10) var(--space-12) var(--space-10) var(--space-16);
    border-radius: var(--radius-10);
    background: var(--ink);
    color: var(--surface);
    box-shadow: var(--shadow-toast);
  }
  .msg {
    min-width: 0;
    line-height: 1.5;
  }
  .act {
    flex: 0 0 auto;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--accent-on);
    font-weight: 600;
    cursor: pointer;
    text-decoration: underline;
  }
  .x {
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--muted-2);
    cursor: pointer;
  }
</style>
