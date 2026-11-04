<div class="page">
  <header class="head">
    <h1>アカウントと認証</h1>
  </header>
  <p class="lead">
    ここに登録した全アカウントを「自分」として扱います（<code>assignee:@me</code
    >）。トークンはこのマシンにだけ保存し、Store には書きません。
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
  code {
    font-family: var(--font-mono);
  }
  .lead {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
</style>
