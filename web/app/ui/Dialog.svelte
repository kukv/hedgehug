<script lang="ts" module>
  // Dialogs that are open, the top one last. Only the top one answers Esc and Tab.
  const stack: symbol[] = [];

  const FOCUSABLE =
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
</script>

<script lang="ts">
  import { type Snippet, tick } from "svelte";
  import Icon from "../icons/Icon.svelte";
  import CliLine from "./CliLine.svelte";

  type Props = {
    open: boolean;
    title: string;
    width: 560 | 620 | 760;
    cli?: string;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
  };
  let { open, title, width, cli, onclose, children, footer }: Props = $props();

  const uid = $props.id();
  const me = Symbol("dialog");
  let panel: HTMLElement | undefined = $state();
  let body: HTMLElement | undefined = $state();

  function focusables(): HTMLElement[] {
    return panel ? [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)] : [];
  }

  $effect(() => {
    if (!open) return;
    const opener = document.activeElement as HTMLElement | null;
    stack.push(me);
    tick().then(() => {
      const first = body?.querySelector<HTMLElement>(FOCUSABLE) ?? focusables()[0] ?? panel;
      first?.focus();
    });
    return () => {
      const i = stack.indexOf(me);
      if (i >= 0) stack.splice(i, 1);
      opener?.focus?.();
    };
  });

  function onkeydown(e: KeyboardEvent) {
    if (!open || !panel || stack[stack.length - 1] !== me) return;
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      onclose();
      return;
    }
    if (e.key !== "Tab") return;
    const items = focusables();
    if (items.length === 0) {
      e.preventDefault();
      panel.focus();
      return;
    }
    const first = items[0];
    const last = items.at(-1);
    if (!first || !last) return;
    const active = document.activeElement;
    if (!panel.contains(active)) {
      e.preventDefault();
      first.focus();
    } else if (e.shiftKey && active === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }
</script>

<svelte:window {onkeydown} />

{#if open}
  <div class="overlay">
    <div
      class="panel"
      role="dialog"
      aria-modal="true"
      aria-labelledby="{uid}-title"
      tabindex="-1"
      bind:this={panel}
      style:max-width="{width}px"
    >
      <header class="head">
        <h2 id="{uid}-title">{title}</h2>
        <button type="button" class="close" aria-label="閉じる" onclick={onclose}>
          <Icon name="close" size={16} />
        </button>
      </header>
      <div class="body" bind:this={body}>{@render children()}</div>
      {#if cli || footer}
        <footer class="foot">
          {#if cli}
            <div class="cli"><CliLine command={cli} /></div>
          {/if}
          {#if footer}
            <div class="actions">{@render footer()}</div>
          {/if}
        </footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: var(--dialog-top) var(--space-16);
    overflow: auto;
    background: var(--scrim);
  }
  .panel {
    display: flex;
    flex-direction: column;
    width: 100%;
    border-radius: var(--radius-14);
    background: var(--surface);
    box-shadow: var(--shadow-dialog);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-16) var(--space-20) var(--space-12);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-17);
    font-weight: 600;
  }
  .close {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    margin-left: auto;
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    cursor: pointer;
  }
  .close:hover {
    background: var(--hover);
  }
  .body {
    padding: var(--space-4) var(--space-20) var(--space-16);
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-12);
    padding: var(--space-12) var(--space-20);
    border-top: 1px solid var(--line);
    background: var(--sunken);
  }
  .cli {
    flex: 1 1 240px;
    min-width: 0;
  }
  .actions {
    display: flex;
    gap: var(--space-8);
    margin-left: auto;
  }
</style>
