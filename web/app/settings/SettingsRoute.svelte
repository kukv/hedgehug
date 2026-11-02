<script lang="ts">
  import { core } from "../context";
  import { hrefOf, router, type SettingsPage } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import SettingsLayout from "./SettingsLayout.svelte";

  let { page }: { page: SettingsPage } = $props();

  const c = core();
  const dir = useSubscribe(c.stores, () => ({
    names: c.stores.stores().map((s) => s.name),
    loaded: c.stores.loaded(),
  }));
  const first = $derived(dir.current.names[0] ?? null);
  // A Store that is not mounted: said so, like a workspace that is not there.
  const missing = $derived(
    page.store !== "" && dir.current.loaded && !dir.current.names.includes(page.store),
  );

  // /settings names no Store: it is replaced by the first one, once the Stores are read.
  $effect(() => {
    if (page.store !== "" || first === null) return;
    router.go(hrefOf({ name: "settings", page: page.page, store: first }), {
      replace: true,
    });
  });
</script>

{#if missing}
  <div class="missing">
    <b>Store「{page.store}」が見つかりません</b>
    <Button size="sm" onclick={() => router.go(hrefOf({ name: "home" }))}>ホームへ戻る</Button>
  </div>
{:else if page.store !== ""}
  <SettingsLayout {page} />
{/if}

<style>
  .missing {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    align-items: center;
    padding: var(--space-20) var(--gutter);
  }
</style>
