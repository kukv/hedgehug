<script lang="ts">
  import CliLine from "../ui/CliLine.svelte";
</script>

<div class="gate" role="alert" data-blocking>
  <span class="logo" aria-hidden="true">hh</span>
  <h1>Hedgehug の画面を開けません</h1>
  <p>この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください。</p>
  <CliLine command="hh ui" />
</div>

<style>
  .gate {
    position: fixed;
    inset: 0;
    z-index: 150;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-12);
    padding: var(--space-20);
    background: var(--sidebar);
    text-align: center;
  }
  .logo {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--logo-size);
    height: var(--logo-size);
    border-radius: var(--radius-8);
    background: var(--ink);
    color: var(--surface);
    font-family: var(--font-mono);
    font-weight: 500;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  p {
    margin: 0;
    max-width: 440px;
    color: var(--ink-2);
    line-height: 1.7;
  }
</style>
