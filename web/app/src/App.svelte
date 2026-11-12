<script lang="ts">
  import { AppFrame, Toasts } from "@hedgehug/ui";
  import CreateItemDialog from "./screens/item/CreateItemDialog.svelte";
  import DetailPanel from "./screens/item/DetailPanel.svelte";
  import CreateProjectDialog from "./screens/project/CreateProjectDialog.svelte";
  import CreateWorkspaceDialog from "./screens/workspace-settings/CreateWorkspaceDialog.svelte";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { catalog, itemDialog, links, screen, shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();

  // C キーで作成ダイアログを開く。入力中とダイアログを開いているときは使えない(02「開き方と初期値」)。
  function onkeydown(event: KeyboardEvent) {
    if (event.key.toLowerCase() !== "c" || event.ctrlKey || event.metaKey || event.altKey) return;
    const t = event.target as HTMLElement | null;
    const typing = t?.closest("input, textarea, select, [contenteditable='true']");
    if (typing || document.querySelector('[aria-modal="true"]')) return;
    event.preventDefault();
    void itemDialog.open();
  }
</script>

<svelte:window {onkeydown} />

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Sidebar stores={$stores} current={$current} {links} />
  {/snippet}
  <Screen current={$current} stores={$stores} version={$version} ondrawer={shell.openDrawer} />
  <DetailPanel />
</AppFrame>
<CreateProjectDialog />
<CreateWorkspaceDialog />
<CreateItemDialog />
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
