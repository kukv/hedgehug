<script lang="ts">
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import CreateStore from "../create/CreateStore.svelte";
  import StoreDiscoverDialog from "../stores/StoreDiscoverDialog.svelte";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreAddDialog from "./StoreAddDialog.svelte";
  import StoreCard from "./StoreCard.svelte";
  import StoreRemoveDialog from "./StoreRemoveDialog.svelte";

  const c = core();
  const dir = useSubscribe(c.stores, () => ({
    list: c.stores.stores(),
    loaded: c.stores.loaded(),
    error: c.stores.error(),
  }));

  let discovering = $state(false);
  let adding = $state(false);
  let creating = $state(false);
  let removing = $state<string | null>(null);

  const shown = $derived(dir.current.list.filter((s) => !s.hidden).length);
</script>

<div class="page">
  <header class="head">
    <h1>マウント中の Store</h1>
    <span class="spacer"></span>
    <Button onclick={() => (discovering = true)}>Store を探す</Button>
    <Button onclick={() => (adding = true)}>URL で追加</Button>
    <Button variant="primary" onclick={() => (creating = true)}>Store を作成</Button>
  </header>
  <p class="lead">
    このマシンで扱う Store です。見える範囲は各リポジトリのアクセス権で決まります。「表示」を外した
    Store のワークスペースは、サイドバーにもビューにも出しません。
  </p>

  {#if dir.current.error}
    <FormError message="Store を読み込めません · {errorMessage(dir.current.error)}" />
    <div>
      <Button onclick={() => c.stores.reload().catch(() => undefined)}>もう一度読み込む</Button>
    </div>
  {/if}

  {#if !dir.current.loaded}
    {#if !dir.current.error}
      <p class="note">読み込み中…</p>
    {/if}
  {:else if dir.current.list.length === 0}
    <p class="note">マウント中の Store はありません</p>
  {:else}
    <ul class="cards">
      {#each dir.current.list as s (s.name)}
        <li>
          <StoreCard store={s} lastVisible={shown === 1} onremove={() => (removing = s.name)} />
        </li>
      {/each}
    </ul>
  {/if}
</div>

<StoreDiscoverDialog open={discovering} onclose={() => (discovering = false)} />
<StoreAddDialog open={adding} onclose={() => (adding = false)} />
<CreateStore open={creating} onclose={() => (creating = false)} />
<StoreRemoveDialog name={removing} onclose={() => (removing = null)} />

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
  .spacer {
    flex: 1;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .lead,
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .cards {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    margin: 0;
    padding: 0;
    list-style: none;
  }
</style>
