<script lang="ts">
  interface Props {
    checked?: boolean;
    label: string;
    disabled?: boolean;
    title?: string;
  }

  let { checked = $bindable(false), label, disabled = false, title }: Props = $props();
</script>

<span class="switch" {title}>
  <input
    type="checkbox"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    {disabled}
    bind:checked
  >
  <span></span>
</span>

<style>
  .switch {
    position: relative;
    width: 34px;
    height: 20px;
    flex: none;
    display: inline-block;
  }
  input {
    opacity: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    position: absolute;
    cursor: pointer;
    z-index: 1;
  }
  input:disabled {
    cursor: not-allowed;
  }
  span span {
    position: absolute;
    inset: 0;
    border-radius: 999px;
    background: var(--line-input);
    transition: background 0.15s;
  }
  span span::after {
    content: "";
    position: absolute;
    width: 16px;
    height: 16px;
    left: 2px;
    top: 2px;
    border-radius: 50%;
    background: var(--surface);
    transition: transform 0.15s;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  }
  input:checked + span {
    background: var(--accent);
  }
  input:checked + span::after {
    transform: translateX(14px);
  }
  input:focus-visible + span {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  input:disabled + span {
    opacity: 0.45;
  }
</style>
