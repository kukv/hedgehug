<script lang="ts">
  import { core } from "../context";
  import { hrefOf, router, type SettingsPage } from "../router";
  import { useSubscribe } from "../svelte-store";
  import SettingsLayout from "./SettingsLayout.svelte";

  let { page }: { page: SettingsPage } = $props();

  const c = core();
  const first = useSubscribe(c.stores, () => c.stores.stores()[0]?.name ?? null);

  // /settings names no Store: it is replaced by the first one, once the Stores are read.
  $effect(() => {
    if (page.store !== "" || first.current === null) return;
    router.go(hrefOf({ name: "settings", page: page.page, store: first.current }), {
      replace: true,
    });
  });
</script>

{#if page.store !== ""}
  <SettingsLayout {page} />
{/if}
