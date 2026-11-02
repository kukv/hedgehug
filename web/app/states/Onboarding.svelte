<script lang="ts">
  import { cliStoreInit } from "../../core/cli";
  import CreateStore from "../create/CreateStore.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";

  let creating = $state(false);
</script>

<div class="onb">
  <div class="brand">
    <span class="logo" aria-hidden="true">hh</span>
    <span class="welcome">Hedgehug へようこそ</span>
  </div>

  <section class="card">
    <h1>最初の Store を作ります</h1>
    <p>
      Hedgehug はタスクを Store に保存します。Store はタスク専用の git リポジトリです。名前と ID
      の接頭辞を決めて作ります。remote はあれば指定でき、なくてもこのマシンだけで使えます。
    </p>
    <div class="actions">
      <Button variant="primary" onclick={() => (creating = true)}>Store を作る</Button>
    </div>
    <div class="cli">
      <span>端末では</span>
      <CliLine command={cliStoreInit({ name: "my-tasks", prefix: "ME" })} />
    </div>
  </section>
</div>

<CreateStore open={creating} onclose={() => (creating = false)} />

<style>
  .onb {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-20);
    padding: calc(var(--dialog-top) + var(--space-4)) var(--space-16);
    overflow: auto;
    background: var(--sidebar);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: var(--space-10);
  }
  .logo {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: var(--radius-8);
    background: var(--ink);
    color: var(--surface);
    font-family: var(--font-mono);
    font-size: var(--fs-14);
    font-weight: 500;
  }
  .welcome {
    font-size: var(--fs-17);
    font-weight: 600;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    width: 100%;
    max-width: 760px;
    padding: var(--space-20) 26px;
    border-radius: var(--radius-14);
    background: var(--surface);
    box-shadow: var(--shadow-card);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  p {
    margin: 0;
    color: var(--ink-2);
    line-height: 1.7;
  }
  .actions {
    display: flex;
  }
  .cli {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    color: var(--muted);
    font-size: var(--fs-12);
  }
</style>
