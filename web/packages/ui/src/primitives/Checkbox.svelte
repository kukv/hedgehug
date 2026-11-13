<script lang="ts">
  interface Props {
    checked?: boolean;
    // label は、チェックボックスの右に出す文字(例「続けて作成」)。
    label: string;
    disabled?: boolean;
    title?: string;
    // onchange を渡したら、押しても見た目を変えず、次の値を渡すだけにする(保存のあとの読み直しで checked が変わる)。
    onchange?: (checked: boolean) => void;
  }

  let { checked = $bindable(false), label, disabled = false, title, onchange }: Props = $props();

  function changed(e: Event & { currentTarget: HTMLInputElement }) {
    const next = e.currentTarget.checked;
    if (!onchange) {
      checked = next;
      return;
    }
    e.currentTarget.checked = checked;
    onchange(next);
  }
</script>

<label class="check" class:disabled {title}>
  <input type="checkbox" {disabled} {checked} onchange={changed}>
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
