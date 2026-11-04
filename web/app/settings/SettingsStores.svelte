<div class="page">
  <header class="head">
    <h1>マウント中の Store</h1>
  </header>
  <p class="lead">
    このマシンで扱う Store です。見える範囲は各リポジトリのアクセス権で決まります。「表示」を外した
    Store のワークスペースは、サイドバーにもビューにも出しません。
  </p>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    max-width: 48rem;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-12);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .lead {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
</style>
