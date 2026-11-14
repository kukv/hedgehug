<script lang="ts">
  import { tick } from "svelte";
  import type { PaletteView } from "../types";
  import PaletteRow from "./PaletteRow.svelte";

  // コマンドパレット(02「コマンドパレット」、01 キーボード)。状態は core の createPalette が持ち、ここは描くことと
  // キーを渡すことだけ。view が null なら閉じている。
  interface Props {
    view: PaletteView | null;
    // mac なら Cmd+K・Cmd+Enter、ほかは Ctrl+K・Ctrl+Enter。
    mac: boolean;
    onopen: () => void;
    oninput: (text: string) => void;
    onmove: (dir: "up" | "down") => void;
    // onchoose は、index の行(省くと選んでいる行)を実行する。newTab は Ctrl+Enter(Cmd+Enter)。
    onchoose: (index: number | undefined, newTab: boolean) => void;
    onclose: () => void;
  }

  let { view, mac, onopen, oninput, onmove, onchoose, onclose }: Props = $props();
  const ids = $props.id();
  let input = $state<HTMLInputElement>();
  let list = $state<HTMLElement>();
  const open = $derived(view !== null);
  const modKey = (e: KeyboardEvent) => (mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey);
  // 日本語入力の変換中の ↑↓・Enter・Esc は、候補の選択・変換の確定・取り消しに使う(01「Esc」)。
  const composing = (e: KeyboardEvent) => e.isComposing || e.keyCode === 229;

  // Ctrl+K(Cmd+K)は入力中でも使える。ダイアログ(パレット自身を含む)を開いているときは使えない(01 キーボード)。
  // capture の段階で受けるので、入力欄やツリーが先に止めても届く。
  // 開いているあいだの Esc もここで受けて止め、ほかの Esc の受け手(詳細パネルなど)へ届けない(01「Esc」の順)。
  function onkeydowncapture(event: KeyboardEvent) {
    if (open && event.key === "Escape" && !composing(event)) {
      event.preventDefault();
      event.stopPropagation();
      onclose();
      return;
    }
    if (event.key.toLowerCase() !== "k" || !modKey(event) || event.altKey || event.shiftKey) return;
    if (document.querySelector('[aria-modal="true"]')) return;
    event.preventDefault();
    onopen();
  }

  function onkeydown(event: KeyboardEvent) {
    if (!open || composing(event)) return;
    if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      event.preventDefault();
      onmove(event.key === "ArrowUp" ? "up" : "down");
    } else if (event.key === "Enter") {
      event.preventDefault();
      onchoose(undefined, modKey(event));
    }
  }

  // 背景(暗くしたところ)を押したときだけ閉じる。
  function onmousedown(event: MouseEvent) {
    if (event.target === event.currentTarget) onclose();
  }

  // 開いたら入力欄にフォーカスを置き、閉じたら開く前の場所に戻す(メニューと同じ。menuFocus)。候補が別の
  // ダイアログを開いたとき、画面を移って元の場所がなくなったとき、ほかの場所にフォーカスが移ったときは戻さない。
  $effect(() => {
    if (!open) return;
    const before = document.activeElement;
    input?.focus();
    return () => {
      void tick().then(() => {
        if (!(before instanceof HTMLElement) || !before.isConnected) return;
        if (document.querySelector('[aria-modal="true"]')) return;
        const lost = document.activeElement === null || document.activeElement === document.body;
        if (lost) before.focus({ preventScroll: true });
      });
    };
  });

  // 選んでいる行が見えるようにスクロールする。
  $effect(() => {
    void view?.selected;
    list?.querySelector(".selected")?.scrollIntoView?.({ block: "nearest" });
  });
</script>

<svelte:window {onkeydowncapture} {onkeydown} />

{#if view}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="scrim" {onmousedown}>
    <!-- data-palette は、ツリーがフォーカスの戻り先を覚えるための印(treeKeyboard の inMenu) -->
    <div class="palette" role="dialog" aria-modal="true" aria-label="コマンドパレット" data-palette>
      <input
        bind:this={input}
        class="field"
        type="text"
        role="combobox"
        aria-label="コマンドパレット"
        aria-expanded="true"
        aria-controls="{ids}-list"
        aria-activedescendant={view.selected >= 0 ? `${ids}-${view.selected}` : undefined}
        placeholder="ID・タイトル・コマンドを入力"
        autocomplete="off"
        value={view.input}
        oninput={(e) => oninput(e.currentTarget.value)}
      >
      <div class="list" role="listbox" id="{ids}-list" aria-label="候補" bind:this={list}>
        {#each view.rows as row, i (i)}
          <PaletteRow
            {row}
            id="{ids}-{i}"
            selected={i === view.selected}
            dot={row.kind === "head" && row.text === view.selectedHead?.text
              ? view.selectedHead.storeColor
              : undefined}
            onchoose={() => onchoose(i, false)}
          />
        {/each}
      </div>
      <div class="foot">{view.footer}</div>
    </div>
  </div>
{/if}

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: var(--scrim);
    /* ダイアログ(55)と同じ高さ。パレットを開いているあいだは、ほかのダイアログを開かない(02)。 */
    z-index: 55;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: max(64px, env(safe-area-inset-top, 0px)) 16px 16px;
  }
  .palette {
    background: var(--surface);
    border-radius: 12px;
    box-shadow: var(--shadow-dialog);
    width: min(600px, 100%);
    max-height: calc(100vh - 96px);
    display: flex;
    flex-direction: column;
  }
  .field {
    margin: 10px;
    padding: 8px 10px;
    font-size: 15px;
  }
  .list {
    overflow-y: auto;
    padding: 0 6px 6px;
  }
  .foot {
    padding: 8px 14px;
    border-top: 1px solid var(--line);
    font-size: 11.5px;
    color: var(--muted);
  }
</style>
