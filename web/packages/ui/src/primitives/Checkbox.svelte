<script lang="ts">
  interface Props {
    checked?: boolean;
    // label は、チェックボックスの右に出す文字(例「続けて作成」)。
    label: string;
    disabled?: boolean;
    title?: string;
  }

  let { checked = $bindable(false), label, disabled = false, title }: Props = $props();
</script>

<label class="check" class:disabled {title}>
  <input type="checkbox" {disabled} bind:checked>
  <span>{label}</span>
</label>

<style>
  .check {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--ink-2);
    font-size: 12.5px;
    cursor: pointer;
  }
  input {
    margin: 0;
    width: 14px;
    height: 14px;
    accent-color: var(--accent);
    cursor: inherit;
  }
  .disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
