<script lang="ts">
  interface Props {
    // text は、帯に出す文(01「閲覧のみの Store」の文。core が作る)。
    text: string;
    // inset は、詳細パネルの中のように、角を丸めて余白の中に置く。
    inset?: boolean;
  }

  let { text, inset = false }: Props = $props();
</script>

<p class="band" class:inset role="note">{text}</p>

<style>
  .band {
    margin: 0;
    padding: 8px 24px;
    background: var(--c-amber-bg);
    color: var(--c-amber-fg);
    font-size: 12.5px;
    border-bottom: 1px solid var(--line);
  }
  .inset {
    border-radius: 8px;
    border: 0;
    padding: 8px 12px;
  }
</style>
