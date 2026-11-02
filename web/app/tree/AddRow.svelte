<script lang="ts">
  import Icon from "../icons/Icon.svelte";

  type Props = {
    editing: boolean;
    depth?: number;
    where: string; // "release / リリース準備 v2.4"
    kindLabel: string; // "タスク"
    ready?: boolean; // false while the schema of the store is not read: the title waits
    onstart: () => void;
    // false: the task was not made, so the title stays
    onsubmit: (title: string) => boolean | Promise<boolean>;
    oncancel: () => void;
  };
  let {
    editing,
    depth = 0,
    where,
    kindLabel,
    ready = true,
    onstart,
    onsubmit,
    oncancel,
  }: Props = $props();

  let title = $state("");
  let busy = $state(false);

  async function submit() {
    const t = title.trim();
    if (t === "") {
      oncancel();
      return;
    }
    if (busy || !ready) return;
    busy = true;
    try {
      if ((await onsubmit(t)) !== false) title = "";
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing) {
      e.preventDefault();
      void submit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      oncancel();
    }
  }

  function focusOnMount(el: HTMLInputElement) {
    el.focus();
  }
</script>

{#if editing}
  <div class="add editing" style:padding-left="{2.4 + depth * 1.6}rem">
    <Icon name="plus" size={14} />
    <input
      class="title"
      type="text"
      aria-label="新しいタスクのタイトル"
      placeholder={ready ? "タイトルを入力して Enter（Esc で閉じる）" : "読み込み中…"}
      readonly={!ready}
      bind:value={title}
      {onkeydown}
      {@attach focusOnMount}
    >
    <span class="where">→ {where} · 種類: {kindLabel}</span>
  </div>
{:else}
  <button class="add" type="button" onclick={onstart}>
    <Icon name="plus" size={14} />
    <span>タスクを追加</span>
    <span class="where">→ {where} · 種類: {kindLabel}</span>
  </button>
{/if}

<style>
  .add {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
    width: 100%;
    min-height: 2.375rem;
    padding: 0 0.75rem 0 2.375rem;
    border: var(--hairline) dashed var(--line-input);
    border-radius: var(--radius-4);
    background: transparent;
    color: var(--muted);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .add:hover {
    background: var(--sunken);
  }
  .editing {
    cursor: text;
    background: var(--surface);
    border-style: solid;
    border-color: var(--accent);
  }
  .title {
    flex: 1 1 12rem;
    min-width: 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--ink);
    font: inherit;
  }
  .where {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 0.72rem;
    color: var(--muted);
  }
</style>
