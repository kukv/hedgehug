<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    rows?: number;
    placeholder?: string;
    disabled?: boolean;
    title?: string;
    // focusOnMount は、出たときにフォーカスを置き、キャレットを末尾に置く(Input と同じ理由で autofocus は使わない)。
    focusOnMount?: boolean;
    onkeydown?: (event: KeyboardEvent) => void;
  }

  let {
    value = $bindable(""),
    label,
    rows = 4,
    placeholder,
    disabled = false,
    title,
    focusOnMount = false,
    onkeydown,
  }: Props = $props();

  let element: HTMLTextAreaElement | undefined = $state();

  $effect(() => {
    if (!focusOnMount || !element) return;
    element.focus();
    element.setSelectionRange(element.value.length, element.value.length);
  });

  // focus は、確認で「キャンセル」を押したあとに、入力欄へフォーカスを戻す。
  export function focus() {
    element?.focus();
  }
</script>

<textarea
  bind:this={element}
  class="textarea"
  aria-label={label}
  {rows}
  {placeholder}
  {disabled}
  {title}
  {onkeydown}
  bind:value
></textarea>

<style>
  .textarea {
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 6px 10px;
    min-width: 0;
    resize: vertical;
  }
  .textarea:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .textarea:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
