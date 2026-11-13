<script lang="ts">
  import { hasSyncError, isSyncAllKey, offlineBand, syncCard } from "@hedgehug/core";
  import { AppFrame, OfflineBand, Toasts } from "@hedgehug/ui";
  import CreateItemDialog from "./screens/item/CreateItemDialog.svelte";
  import DeleteItemDialog from "./screens/item/DeleteItemDialog.svelte";
  import DetailPanel from "./screens/item/DetailPanel.svelte";
  import DiscardDialog from "./screens/notes/DiscardDialog.svelte";
  import CreateProjectDialog from "./screens/project/CreateProjectDialog.svelte";
  import CreateWorkspaceDialog from "./screens/workspace-settings/CreateWorkspaceDialog.svelte";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import {
    catalog,
    itemDialog,
    links,
    online,
    screen,
    shell,
    sync,
    syncNow,
    system,
    toasts,
  } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();

  const syncStatus = sync.status;
  const isOnline = online;

  // オフラインの帯は、同期カードを出す条件のときだけ(06「オフライン」。決定 S20)。
  const band = $derived(
    !$isOnline && syncCard($syncStatus, $syncNow) ? offlineBand($syncStatus?.auto ?? true) : null,
  );
  const alert = $derived(hasSyncError($syncStatus));

  // Shift+Ctrl+S(Shift+Cmd+S)は、入力中とダイアログを開いているときも使える(06。決定 S17)。capture の
  // 段階で受けるので、入力欄やダイアログが先に止めても届く。
  $effect(() => {
    const onSyncKey = (event: KeyboardEvent) => {
      if (!isSyncAllKey(event)) return;
      event.preventDefault();
      event.stopPropagation();
      void sync.syncAll();
    };
    window.addEventListener("keydown", onSyncKey, { capture: true });
    return () => window.removeEventListener("keydown", onSyncKey, { capture: true });
  });

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
  {#if band}
    <OfflineBand text={band} />
  {/if}
  <Screen
    current={$current}
    stores={$stores}
    version={$version}
    {alert}
    ondrawer={shell.openDrawer}
  />
  <DetailPanel />
</AppFrame>
<CreateProjectDialog />
<CreateWorkspaceDialog />
<CreateItemDialog />
<DeleteItemDialog />
<DiscardDialog />
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
