<script lang="ts">
  import { colors } from "@hedgehug/core";

  // 色のボタンと 18 色のメニュー(03「色」)。現在の色は文字を足さず、aria-checked と輪で示す。
  interface Props {
    color: string;
    name: string;
    disabled?: boolean;
    title?: string | undefined;
    onselect: (color: string) => void;
  }

  let { color, name, disabled = false, title, onselect }: Props = $props();

  let at = $state<{ x: number; y: number } | null>(null);
  let element: HTMLDivElement | undefined = $state();

  function open(event: MouseEvent) {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    at = { x: rect.left, y: rect.bottom + 4 };
  }
  function pick(id: string) {
    at = null;
    if (id !== color) onselect(id);
  }
  function onpointerdown(event: PointerEvent) {
    if (at && element && !element.contains(event.target as Node)) at = null;
  }
  function onkeydown(event: KeyboardEvent) {
    if (at && event.key === "Escape") at = null;
  }
</script>

<svelte:window {onpointerdown} {onkeydown} />

<button
  type="button"
  class="swatch"
  aria-label="{name}の色"
  {disabled}
  title={title ?? colors.find((c) => c.id === color)?.label}
  style:background="var(--c-{color}-dot)"
  onclick={open}
></button>
{#if at}
  <div class="pop" role="menu" style:left="{at.x}px" style:top="{at.y}px" bind:this={element}>
    {#each colors as c (c.id)}
      <button
        type="button"
        role="menuitemradio"
        aria-checked={c.id === color}
        aria-label={c.label}
        title={c.label}
        class="swatch choice"
        class:current={c.id === color}
        style:background="var(--c-{c.id}-dot)"
        onclick={() => pick(c.id)}
      ></button>
    {/each}
  </div>
{/if}

<style>
  .swatch {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    border: 1px solid var(--line-input);
    padding: 0;
    flex: none;
  }
  .swatch:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .pop {
    position: fixed;
    z-index: 60;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: 10px;
    box-shadow: var(--shadow-pop);
    padding: 8px;
    display: grid;
    grid-template-columns: repeat(6, 22px);
    gap: 8px;
  }
  .choice {
    width: 22px;
    height: 22px;
  }
  .current {
    outline: 2px solid var(--ink);
    outline-offset: 2px;
  }
</style>
