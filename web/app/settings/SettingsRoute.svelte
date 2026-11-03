<script lang="ts">
  import { core } from "../context";
  import { hrefOf, router, type SettingsPage } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import SettingsGeneral from "./SettingsGeneral.svelte";
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
  // A Store that is not mounted: said so, like a workspace that is not there.
  const missing = $derived(
    page.page === "workspaces" && dir.current.loaded && !dir.current.names.includes(page.store),
  );

  // /settings is the same page as /settings/general: the address says so.
  $effect(() => {
    if (page.page !== "general" || location.pathname === "/settings/general") return;
    router.replace(hrefOf({ name: "settings", page: "general" }));
  });
</script>

{#if missing}
  <Empty
    variant="notfound"
    {...missingProps}
    name={page.page === "workspaces" ? page.store : ""}
    onHome={() => router.go(hrefOf({ name: "home" }))}
  />
{:else}
  <!-- one layout for every page: the nav stays mounted when the page changes -->
  <SettingsLayout {page}>
    {#if page.page === "general"}
      <SettingsGeneral />
    {:else}
      <SettingsWorkspaces store={page.store} />
    {/if}
  </SettingsLayout>
{/if}
