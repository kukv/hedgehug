<script lang="ts">
  import Button from "../primitives/Button.svelte";
  import Dialog from "../primitives/Dialog.svelte";

  // 確認のダイアログ。本文は 1 行を 1 段落にする。ok は実行のボタンの文言、danger なら警告色。
  interface Props {
    title: string;
    lines: string[];
    ok: string;
    danger?: boolean;
    onok: () => void;
    oncancel: () => void;
  }

  let { title, lines, ok, danger = false, onok, oncancel }: Props = $props();
</script>

<Dialog open {title} onclose={oncancel}>
  {#each lines as line, i (i)}
    <p class="line">{line}</p>
  {/each}
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={oncancel}>キャンセル</Button>
    <Button variant={danger ? "danger" : "primary"} onclick={onok}>{ok}</Button>
  {/snippet}
</Dialog>

<style>
  .line {
    margin: 0;
  }
  .grow {
    flex: 1;
  }
</style>
