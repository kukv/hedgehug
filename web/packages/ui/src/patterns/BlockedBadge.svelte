<script lang="ts">
  // 見た目だけで、props はない。script がないと svelte-check が型宣言を見つけられない。
</script>

<span class="blocked">ブロック中</span>

<style>
  .blocked {
    display: inline-flex;
    align-items: center;
    height: 18px;
    padding: 0 6px;
    border-radius: 9px;
    background: var(--danger-weak);
    color: var(--danger);
    font-size: 11px;
    font-weight: 600;
    white-space: nowrap;
  }
</style>
