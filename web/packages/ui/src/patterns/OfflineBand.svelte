<script lang="ts">
  // オフラインの帯(06「オフライン」)。文は core の offlineBand が作る。
  interface Props {
    text: string;
  }

  let { text }: Props = $props();
</script>

<p class="band" role="status">{text}</p>

<style>
  .band {
    margin: 0;
    padding: 8px 24px;
    background: var(--c-amber-bg);
    color: var(--c-amber-fg);
    font-size: 12.5px;
  }
</style>
