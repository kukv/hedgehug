<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    // type は、入力の種類。date は日付の入力欄(値は YYYY-MM-DD)。
    type?: "text" | "date";
    placeholder?: string;
    size?: "md" | "sm";
    mono?: boolean;
    // heading は、詳細パネルのタイトルのような、枠のない大きな 1 行の入力欄にする。
    heading?: boolean;
    disabled?: boolean;
    title?: string;
    id?: string;
    // focusOnMount は、出たときにフォーカスを置く(ダイアログのタイトルの欄など)。autofocus という名前は、
    // 使う側のマークアップで Biome の a11y/noAutofocus に当たるので使わない。
    focusOnMount?: boolean;
    onblur?: (event: FocusEvent) => void;
    onkeydown?: (event: KeyboardEvent) => void;
  }

  let {
    value = $bindable(""),
    label,
    type = "text",
    placeholder,
    size = "md",
    mono = false,
    heading = false,
    disabled = false,
    title,
    id,
    focusOnMount = false,
    onblur,
    onkeydown,
  }: Props = $props();

  let element: HTMLInputElement | undefined = $state();

  // autofocus 属性は a11y の警告になるので、出たときに focus を呼ぶ。
  $effect(() => {
    if (focusOnMount) element?.focus();
  });
</script>

<input
  bind:this={element}
  class="input {size}"
  class:mono
  class:heading
  aria-label={label}
  {type}
  {id}
  {placeholder}
  {disabled}
  {title}
  {onblur}
  {onkeydown}
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
  .heading {
    height: auto;
    border-color: transparent;
    border-radius: 6px;
    padding: 4px 6px;
    margin: 0 -6px;
    background: transparent;
    width: calc(100% + 12px);
    font-size: 18px;
    line-height: 1.4;
  }
  .heading:hover:not(:disabled) {
    border-color: var(--line-input);
  }
  .heading:disabled {
    opacity: 1;
    cursor: default;
  }
</style>
