<script lang="ts">
  import Button from "../primitives/Button.svelte";

  // 一覧の上に出す、選んでいる行のバー(F2b1-5)。件数の文と「一括削除…」「選択を解除」。
  interface Props {
    // text は、件数の文(core の selectedText が作る)。
    text: string;
    // readOnly は、選んだ行がすべて閲覧のみの Store のもの。「一括削除…」だけを無効にする(F2b1-4)。
    readOnly: boolean;
    ondelete: () => void;
    onclear: () => void;
  }

  let { text, readOnly, ondelete, onclear }: Props = $props();

  // ui は core を読めないので、core の readOnlyNote と同じ文をここに書く。
  const readOnlyNote = "閲覧のみの Store です";
</script>

<div class="bar">
  <span class="text">{text}</span>
  <Button
    size="sm"
    disabled={readOnly}
    title={readOnly ? readOnlyNote : undefined}
    onclick={ondelete}
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
