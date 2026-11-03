<script lang="ts">
  import { untrack } from "svelte";
  import Icon from "../icons/Icon.svelte";
  import Menu from "../ui/Menu.svelte";
  import ChipsField from "./ChipsField.svelte";
  import type { Option } from "./field-types";

  let {
    editor,
    value,
    label,
    options = [],
    nullable = true,
    pending = false,
    readonly = false,
    size = "normal",
    placeholder = "",
    transform,
    invalidMessage = "",
    hintFor,
    onedit,
    oncommit,
  }: {
    editor: "text" | "number" | "date" | "select" | "chips";
    value: unknown;
    label: string;
    options?: Option[];
    nullable?: boolean;
    pending?: boolean;
    readonly?: boolean;
    size?: "normal" | "title";
    placeholder?: string;
    transform?: (text: string) => string | null;
    invalidMessage?: string;
    hintFor?: (text: string) => string | null;
    onedit: (v: unknown) => void;
    oncommit: () => void;
  } = $props();

  const asText = (v: unknown) => (v === null || v === undefined ? "" : String(v));

  // The text the user is typing. An outside change is taken only when nothing is pending and the input has no focus.
  let local = $state(untrack(() => asText(value)));
  // $state, so that leaving the input runs the effect again: a value kept while a save was pending is replaced then.
  let focused = $state(false);
  $effect(() => {
    const v = asText(value);
    if (!focused && !pending) local = v;
  });

  function oninput(e: Event & { currentTarget: HTMLInputElement }) {
    local = e.currentTarget.value;
    if (editor === "number") {
      if (local.trim() === "") onedit(null);
      else if (!Number.isNaN(Number(local))) onedit(Number(local));
      return;
    }
    onedit(local === "" && nullable ? null : local);
  }

  function ondate(e: Event & { currentTarget: HTMLInputElement }) {
    const v = e.currentTarget.value;
    onedit(v === "" ? null : v);
    oncommit();
  }

  // The menu of a select: the button shows the current choice and opens ui/Menu under itself.
  let trigger = $state<HTMLButtonElement | null>(null);
  let open = $state(false);
  const items = $derived([
    ...(nullable ? [{ value: "", label: "—" }] : []),
    ...options.map((o) => ({
      value: o.value,
      label: o.label,
      disabled: o.disabled,
      hint: o.title,
    })),
  ]);
  const current = $derived(items.find((o) => o.value === asText(value))?.label ?? asText(value));

  function choose(v: string) {
    open = false;
    trigger?.focus();
    onedit(v === "" ? null : v);
    oncommit();
  }

  function onchips(list: string[]) {
    onedit(list);
    oncommit();
  }
</script>

{#if editor === "select"}
  <button
    type="button"
    class="pick"
    bind:this={trigger}
    aria-label={label}
    aria-haspopup="menu"
    aria-expanded={open}
    disabled={readonly}
    onclick={() => (open = !open)}
  >
    <span class="cur">{current || "—"}</span>
    {#if !readonly}
      <Icon name="chevron-down" size={12} />
    {/if}
  </button>
  {#if open}
    <Menu
      anchor={trigger}
      {items}
      {label}
      selected={asText(value)}
      onselect={choose}
      onclose={() => {
        open = false;
        trigger?.focus();
      }}
    />
  {/if}
{:else if editor === "chips"}
  <ChipsField
    values={Array.isArray(value) ? (value as string[]) : value ? [String(value)] : []}
    {label}
    options={options.map((o) => o.value)}
    {placeholder}
    {readonly}
    {transform}
    {invalidMessage}
    {hintFor}
    onchange={onchips}
  />
{:else if editor === "date"}
  <input type="date" aria-label={label} value={asText(value)} {readonly} onchange={ondate}>
{:else}
  <input
    type={editor === "number" ? "number" : "text"}
    class:title={size === "title"}
    aria-label={label}
    {placeholder}
    {readonly}
    value={local}
    {oninput}
    onfocus={() => (focused = true)}
    onblur={() => {
      focused = false;
      oncommit();
    }}
  >
{/if}

<style>
  input,
  .pick {
    width: 100%;
    height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 1px solid transparent;
    border-radius: var(--radius-6);
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  .pick {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    text-align: left;
    cursor: pointer;
  }
  .cur {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  input:hover:not([readonly]),
  .pick:hover:not(:disabled) {
    border-color: var(--line-input);
  }
  input:focus-visible,
  .pick:focus-visible {
    outline: 2px solid var(--accent);
    background: var(--surface);
  }
  input[readonly],
  .pick:disabled {
    color: var(--ink-2);
    cursor: default;
  }
  .title {
    height: auto;
    padding: var(--space-4) var(--space-8);
    font-size: var(--fs-19);
    font-weight: 600;
  }
</style>
