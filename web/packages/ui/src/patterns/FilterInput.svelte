<script lang="ts">
  import type { FilterOption } from "../types";

  // 絞り込み欄(04「絞り込み欄」「候補」)。入力・候補・エラーを描き、キーを渡すだけ。候補の中身は core が作る。
  interface Props {
    value: string;
    placeholder: string;
    options: FilterOption[];
    // errors は、欄の下に 1 行ずつ赤字で出す。
    errors: string[];
    // oninput は、入力した文字とカーソルの位置を渡す。
    oninput: (text: string, caret: number) => void;
    onchoose: (index: number) => void;
    onhelp: () => void;
  }

  let { value, placeholder, options, errors, oninput, onchoose, onhelp }: Props = $props();
  const ids = $props.id();
  let focused = $state(false);
  // closed は、Esc で候補を閉じたあいだ。入力するかフォーカスを入れ直すと開く。
  let closed = $state(false);
  let selected = $state(0);
  const open = $derived(focused && !closed && options.length > 0);
  // 日本語入力の変換中の Enter・Esc は、変換の確定・取り消しに使う(01「Esc」「Enter」)。
  const composing = (e: KeyboardEvent) => e.isComposing || e.keyCode === 229;

  function onkeydown(event: KeyboardEvent) {
    if (composing(event)) return;
    if (event.key === "Escape") {
      // 候補が出ているときだけ止める。ないときは外(詳細パネルなど)へ届ける(01「Esc」の順)。
      if (!open) return;
      event.preventDefault();
      event.stopPropagation();
      closed = true;
    } else if (!open) {
      return;
    } else if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      event.preventDefault();
      const n = options.length;
      selected = (Math.min(selected, n - 1) + (event.key === "ArrowUp" ? n - 1 : 1)) % n;
    } else if (event.key === "Enter") {
      event.preventDefault();
      onchoose(Math.min(selected, options.length - 1));
    }
  }
</script>

<div class="filter">
  <div class="bar">
    <input
      class="field"
      type="text"
      role="combobox"
      aria-label="絞り込み"
      aria-expanded={open}
      aria-controls="{ids}-list"
      aria-activedescendant={open ? `${ids}-${Math.min(selected, options.length - 1)}` : undefined}
      aria-invalid={errors.length > 0 ? "true" : undefined}
      data-filter-input
      {placeholder}
      {value}
      autocomplete="off"
      onfocus={() => {
        focused = true;
        closed = false;
      }}
      onblur={() => (focused = false)}
      oninput={(e) => {
        closed = false;
        selected = 0;
        oninput(
          e.currentTarget.value,
          e.currentTarget.selectionStart ?? e.currentTarget.value.length,
        );
      }}
      {onkeydown}
    >
    <button type="button" class="help" onclick={onhelp}>検索ヘルプ</button>
    {#if open}
      <div class="list" role="listbox" id="{ids}-list" aria-label="候補">
        {#each options as option, i (i)}
          <!-- フォーカスは入力欄に置いたまま。選択は aria-activedescendant で示す -->
          <button
            type="button"
            role="option"
            tabindex="-1"
            id="{ids}-{i}"
            class="option"
            class:selected={i === selected}
            aria-selected={i === selected}
            onmousedown={(e) => e.preventDefault()}
            onclick={() => onchoose(i)}
          >
            {#if option.dot}
              <span class="store-dot" style="--dot: var(--t-{option.dot}-fg)"></span>
            {/if}
            {option.label}
          </button>
        {/each}
      </div>
    {/if}
  </div>
  {#each errors as error, i (i)}
    <div class="error" role="alert">{error}</div>
  {/each}
</div>

<style>
  .filter {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .bar {
    position: relative;
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .field {
    flex: 1;
    min-width: 0;
    padding: 6px 10px;
  }
  .help {
    border: 0;
    background: transparent;
    color: var(--accent);
    white-space: nowrap;
  }
  .list {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    z-index: 20;
    max-height: 280px;
    overflow-y: auto;
    padding: 6px;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: var(--shadow-dialog);
  }
  .option {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    min-height: 30px;
    padding: 4px 10px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    text-align: left;
  }
  .option:hover {
    background: var(--hover);
  }
  .option.selected {
    background: var(--accent-weak);
  }
  .store-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--dot);
  }
  .error {
    font-size: 12px;
    color: var(--danger);
  }
</style>
