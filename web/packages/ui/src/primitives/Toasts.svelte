<script lang="ts">
  import type { Toast } from "../types";

  interface Props {
    toasts: Toast[];
    ondismiss: (id: string) => void;
    onaction?: (id: string) => void;
  }

  let { toasts, ondismiss, onaction }: Props = $props();
</script>

<div class="toasts">
  {#each toasts as toast (toast.id)}
    <div class="toast" class:error={toast.error} role="status">
      <span>{toast.text}</span>
      {#if toast.action}
        <button type="button" class="toast-btn" onclick={() => onaction?.(toast.id)}>
          {toast.action}
        </button>
      {/if}
      <button type="button" class="toast-x" aria-label="閉じる" onclick={() => ondismiss(toast.id)}>
        ×
      </button>
    </div>
  {/each}
</div>

<style>
  .toasts {
    pointer-events: none;
    position: fixed;
    bottom: max(16px, env(safe-area-inset-bottom, 0px));
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 80;
    width: min(560px, calc(100vw - 32px));
  }
  .toast {
    pointer-events: auto;
    background: var(--ink);
    color: var(--surface);
    border-radius: 8px;
    padding: 8px 8px 8px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
    box-shadow: var(--shadow-pop);
    font-size: 12.5px;
  }
  .toast span {
    flex: 1;
    white-space: pre-line;
  }
  .error {
    background: var(--danger);
  }
  .toast-btn {
    border: 0;
    background: transparent;
    color: inherit;
    font-weight: 600;
    text-decoration: underline;
  }
  .toast-x {
    border: 0;
    background: transparent;
    color: inherit;
    opacity: 0.7;
    font-size: 16px;
    width: 26px;
  }
</style>
