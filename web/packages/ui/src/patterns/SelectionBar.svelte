<script lang="ts">
  import Button from "../primitives/Button.svelte";

  // 一覧の上に出す、選んでいる行のバー(F2b1-5)。件数の文と「一括削除…」「選択を解除」。
  interface Props {
    // text は、件数の文(core の selectedText が作る)。
    text: string;
    // readOnlyNote は、選んだ行がすべて閲覧のみの Store のものときに渡す文(core の readOnlyNote)。
    // 渡したら「一括削除…」だけを無効にして、title に出す(F2b1-4)。
    readOnlyNote?: string;
    ondelete: () => void;
    onclear: () => void;
  }

  let { text, readOnlyNote, ondelete, onclear }: Props = $props();
</script>

<div class="bar">
  <span class="text">{text}</span>
  <Button size="sm" disabled={!!readOnlyNote} title={readOnlyNote} onclick={ondelete}
    >一括削除…</Button
  >
  <Button size="sm" onclick={onclear}>選択を解除</Button>
</div>

<style>
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    margin-bottom: 12px;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--accent-faint);
  }
  .text {
    font-size: 13px;
    margin-right: auto;
  }
</style>
