<script lang="ts">
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import { type AppCore, core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { replaceTerm, suggestTerm } from "../list/suggest";
  import CliLine from "../ui/CliLine.svelte";

  type Props = {
    base: string; // the fixed part, as the user would type it; "" for none (every store)
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

  $effect(() => () => clearTimeout(timer));

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

  // "もしかして": the term the server refused, written with the name it is closest to. The names
  // are in the schemas of the stores, which are read when there is an error to explain. A bar
  // drawn without the core (it holds the schemas) shows the error and cannot suggest.
  function appCore(): AppCore | null {
    try {
      return core();
    } catch {
      return null;
    }
  }
  const c = appCore();
  let schemas = $state.raw<SchemaView[]>([]);
  $effect(() => {
    if (!c || !error) {
      schemas = [];
      return;
    }
    let live = true;
    void Promise.all(c.stores.stores().map((s) => c.schema.load(s.name).catch(() => null))).then(
      (all) => {
        if (live) schemas = all.filter((s) => s !== null);
      },
    );
    return () => {
      live = false;
    };
  });
  const fix = $derived.by(() => {
    const to = error ? suggestTerm(error, schemas) : null;
    const q = to ? replaceTerm(applied, error, to) : null;
    return to && q !== null ? { to, q } : null;
  });
</script>

<div class="bar">
  <div class="field" class:bad={message !== null}>
    <Icon name="filter" size={15} />
    {#if base}
      <span class="base" title="この画面で固定のクエリ">{base}</span>
    {/if}
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
    {#if fix}
      <p class="hint">
        <button type="button" class="fix" onclick={() => onchange(fix.q)}>{fix.to}</button>
        のことですか？
      </p>
    {/if}
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
  .hint {
    margin: var(--space-4) 0 0;
    color: var(--ink-2);
    font-size: 0.78rem;
  }
  .fix {
    padding: 0.1rem 0.4rem;
    border: 0;
    border-radius: var(--radius-4);
    background: var(--accent-weak);
    color: var(--accent-ink);
    font-family: var(--font-mono);
    font-size: 0.75rem;
    cursor: pointer;
  }
  .fix:hover {
    background: var(--accent-on);
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
