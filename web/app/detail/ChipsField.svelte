<script lang="ts">
  let {
    values,
    label,
    options = [],
    placeholder = "",
    readonly = false,
    transform,
    invalidMessage = "",
    hintFor,
    onchange,
  }: {
    values: string[];
    label: string;
    options?: string[];
    placeholder?: string;
    readonly?: boolean;
    transform?: (text: string) => string | null;
    invalidMessage?: string;
    hintFor?: (text: string) => string | null;
    onchange: (values: string[]) => void;
  } = $props();

  let text = $state("");
  let error = $state("");
  const listId = `chips-${Math.random().toString(36).slice(2)}`;
  const hint = $derived(text.trim() === "" ? null : (hintFor?.(text.trim()) ?? null));

  function add() {
    const t = text.trim();
    if (t === "") return;
    const v = transform ? transform(t) : t;
    if (v === null) {
      error = invalidMessage;
      return;
    }
    error = "";
    text = "";
    if (!values.includes(v)) onchange([...values, v]);
  }

  function onkeydown(e: KeyboardEvent) {
    // While an IME converts text, Enter confirms the conversion: it is not the end of a chip.
    if (e.isComposing || e.keyCode === 229) return;
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      add();
    } else if (e.key === "Backspace" && text === "" && values.length > 0) {
      onchange(values.slice(0, -1));
    }
  }
</script>

<div class="chips" class:readonly>
  {#each values as v (v)}
    <span class="chip">
      {v}
      {#if !readonly}
        <button
          type="button"
          aria-label="{v} を外す"
          onclick={() => onchange(values.filter((x) => x !== v))}
        >
          ×
        </button>
      {/if}
    </span>
  {/each}
  {#if !readonly}
    <input
      type="text"
      aria-label={label}
      {placeholder}
      list={options.length > 0 ? listId : undefined}
      bind:value={text}
      oninput={() => (error = "")}
      {onkeydown}
      onblur={add}
    >
    {#if options.length > 0}
      <datalist id={listId}>
        {#each options as o (o)}
          <option value={o}></option>
        {/each}
      </datalist>
    {/if}
  {/if}
</div>
{#if hint}
  <p class="hint">{hint}</p>
{/if}
{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-4);
    min-height: var(--control-h-sm);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: 0 var(--space-6);
    border-radius: var(--radius-4);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .chip button {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
  }
  input {
    flex: 1;
    min-width: 6em;
    height: var(--control-h-sm);
    border: 0;
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
    border-radius: var(--radius-4);
  }
  .hint {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .error {
    margin: var(--space-4) 0 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
