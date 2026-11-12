<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    placeholder?: string;
    size?: "md" | "sm";
    mono?: boolean;
    disabled?: boolean;
    title?: string;
  }

  let {
    value = $bindable(""),
    label,
    placeholder,
    size = "md",
    mono = false,
    disabled = false,
    title,
  }: Props = $props();
</script>

<input
  class="input {size}"
  class:mono
  aria-label={label}
  {placeholder}
  {disabled}
  {title}
  bind:value
>

<style>
  .input {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .input:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .input:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .sm {
    height: 28px;
    font-size: 12.5px;
  }
  .mono {
    font-family: var(--font-mono);
  }
</style>
