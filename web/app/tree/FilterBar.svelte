<script lang="ts">
  import { errorMessage } from "../../core/messages";
  import Icon from "../icons/Icon.svelte";
  import CliLine from "../ui/CliLine.svelte";

  type Props = {
    base: string; // the fixed part, as the user would type it
    applied: string; // what is applied now
    count: string; // "16 件 · 中止 1 件を除外"
    cli: string; // the hh ls line
    error: unknown;
    onchange: (q: string) => void;
  };
  let { base, applied, count, cli, error, onchange }: Props = $props();

  let typed = $state("");
  let timer: ReturnType<typeof setTimeout> | undefined;

  // Follow what is applied from outside (the empty state clears it, a new view).
  $effect(() => {
    typed = applied;
  });

  function input(e: Event) {
    typed = (e.currentTarget as HTMLInputElement).value;
    clearTimeout(timer);
    timer = setTimeout(() => onchange(typed.trim()), 300);
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing) {
      clearTimeout(timer);
      onchange(typed.trim());
    }
  }

  const message = $derived(error ? errorMessage(error) : null);
</script>

<div class="bar">
  <div class="field" class:bad={message !== null}>
    <Icon name="filter" size={15} />
    <span class="base" title="この画面で固定のクエリ">{base}</span>
    <label class="sr" for="tree-q">フィルタ（hh ls と同じクエリ）</label>
    <input
      id="tree-q"
      type="text"
      value={typed}
      spellcheck="false"
      autocomplete="off"
      placeholder="例: assignee:@me -status:done"
      oninput={input}
      onkeydown={keydown}
    >
    <span class="count">{count}</span>
    <CliLine command={cli} />
  </div>
  {#if message}
    <p class="error" role="alert">
      {message}
    </p>
  {/if}
</div>

<style>
  .bar {
    padding: 0.75rem 1.75rem;
  }
  .field {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 2.25rem;
    padding: 0 0.375rem 0 0.625rem;
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--muted);
  }
  .field:focus-within {
    border-color: var(--accent);
  }
  .field.bad {
    border-color: var(--danger);
  }
  .base {
    flex: 0 0 auto;
    padding: 0.1rem 0.4rem;
    border-radius: var(--radius-4);
    background: var(--hover);
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: 0.75rem;
  }
  input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: 0.78rem;
  }
  .count {
    flex: 0 0 auto;
    font-size: 0.75rem;
    color: var(--muted);
    white-space: nowrap;
  }
  .error {
    margin: 0.375rem 0 0;
    color: var(--danger);
    font-size: 0.78rem;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }
</style>
