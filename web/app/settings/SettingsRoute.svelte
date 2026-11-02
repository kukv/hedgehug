<script lang="ts">
  import { core } from "../context";
  import { hrefOf, router, type SettingsPage } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import SettingsLayout from "./SettingsLayout.svelte";
  import SettingsWorkspaces from "./SettingsWorkspaces.svelte";

  let { page }: { page: SettingsPage } = $props();

  const c = core();
  const dir = useSubscribe(c.stores, () => ({
    names: c.stores.stores().map((s) => s.name),
    loaded: c.stores.loaded(),
  }));
  // Not written as scope="..." in the markup: Biome reads that name as the HTML attribute.
  const missingProps = { scope: "store" as const };
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
  <Empty
    variant="notfound"
    {...missingProps}
    name={page.store}
    onHome={() => router.go(hrefOf({ name: "home" }))}
  />
{:else if page.store !== ""}
  <SettingsLayout {page}>
    <SettingsWorkspaces store={page.store} />
  </SettingsLayout>
{/if}
