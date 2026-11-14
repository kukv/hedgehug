<script lang="ts">
  // リリースのタブで、リリースがまだないときの表示(05「リリース」)。
</script>

<div class="empty">
  <p class="title">リリースはまだありません</p>
  <p>バージョンを作って Epic・Story・Task・Bug をまとめると、何が残っているかが見えます。</p>
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 48px 16px;
    text-align: center;
    color: var(--muted);
  }
  .empty p {
    margin: 0;
  }
  .empty .title {
    font-size: 15px;
    color: var(--ink);
  }
</style>
