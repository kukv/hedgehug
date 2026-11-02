<script lang="ts">
  import { onDestroy } from "svelte";
  import { copyText } from "./clipboard";

  let { command }: { command: string } = $props();
  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    if (!(await copyText(command))) return;
    copied = true;
    clearTimeout(timer);
    timer = setTimeout(() => {
      copied = false;
    }, 1500);
  }

  onDestroy(() => clearTimeout(timer));
</script>

<button
  type="button"
  class="cli"
  onclick={copy}
  title="クリックでコピー"
  aria-label="コマンドをコピー: {command}"
>
  <span class="text">$ {command}</span>
  <span class="state" aria-live="polite">{copied ? "コピーしました" : ""}</span>
</button>

<style>
  .cli {
    display: inline-flex;
    align-items: center;
    gap: var(--space-8);
    max-width: 100%;
    min-height: var(--control-h-sm);
    padding: 0 var(--space-10);
    border: 0;
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    cursor: pointer;
    text-align: left;
  }
  .cli:hover {
    background: var(--hover);
  }
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .state {
    flex: 0 0 auto;
    color: var(--success);
    font-family: var(--font-sans);
    font-size: var(--fs-12);
  }
</style>
