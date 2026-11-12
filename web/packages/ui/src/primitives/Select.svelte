<script lang="ts">
  import type { SelectOption } from "../types";

  interface Props {
    value?: string;
    label: string;
    options: SelectOption[];
    size?: "md" | "sm";
    disabled?: boolean;
    title?: string;
    // onchange は、選んだ値を渡す(選んだときに保存するため)。
    onchange?: (value: string) => void;
  }

  let {
    value = $bindable(""),
    label,
    options,
    size = "md",
    disabled = false,
    title,
    onchange,
  }: Props = $props();
</script>

<select
  class="select {size}"
  aria-label={label}
  {disabled}
  {title}
  bind:value
  onchange={(event) => onchange?.(event.currentTarget.value)}
>
  {#each options as option (option.value)}
    <option value={option.value}>{option.label}</option>
  {/each}
</select>

<style>
  .select {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .select:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .select:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .sm {
    height: 28px;
    font-size: 12.5px;
  }
</style>
