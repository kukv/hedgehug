<script lang="ts">
  import { AppFrame, Toasts } from "@hedgehug/ui";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { catalog, links, screen, shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();
</script>

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Sidebar stores={$stores} current={$current} {links} />
  {/snippet}
  <Screen current={$current} stores={$stores} version={$version} ondrawer={shell.openDrawer} />
</AppFrame>
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
