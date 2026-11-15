<script lang="ts">
  import { focusesFilter, hasSyncError, isSyncAllKey, offlineBand, syncCard } from "@hedgehug/core";
  import { AppFrame, CommandPalette, OfflineBand, Toasts } from "@hedgehug/ui";
  import "./lastScreen";
  import { myTaskBase } from "./myTasks";
  import { palette } from "./palette";
  import BulkChangeDialog from "./screens/item/BulkChangeDialog.svelte";
  import BulkDeleteDialog from "./screens/item/BulkDeleteDialog.svelte";
  import ChangeKindDialog from "./screens/item/ChangeKindDialog.svelte";
  import CreateItemDialog from "./screens/item/CreateItemDialog.svelte";
  import DeleteItemDialog from "./screens/item/DeleteItemDialog.svelte";
  import DetailPanel from "./screens/item/DetailPanel.svelte";
  import ItemMenuHost from "./screens/item/ItemMenuHost.svelte";
  import MoveMenu from "./screens/item/MoveMenu.svelte";
  import MoveStatusDialog from "./screens/item/MoveStatusDialog.svelte";
  import TransferDialog from "./screens/item/TransferDialog.svelte";
  import DiscardDialog from "./screens/notes/DiscardDialog.svelte";
  import CreateProjectDialog from "./screens/project/CreateProjectDialog.svelte";
  import DeleteProjectDialog from "./screens/project/DeleteProjectDialog.svelte";
  import ProjectTransferDialog from "./screens/project/ProjectTransferDialog.svelte";
  import ProjectMenu from "./screens/tree/ProjectMenu.svelte";
  import RenameProjectDialog from "./screens/tree/RenameProjectDialog.svelte";
  import CreateWorkspaceDialog from "./screens/workspace-settings/CreateWorkspaceDialog.svelte";
  import DeleteWorkspaceDialog from "./screens/workspace-settings/DeleteWorkspaceDialog.svelte";
  import WorkspaceTransferDialog from "./screens/workspace-settings/WorkspaceTransferDialog.svelte";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { isMac } from "./tab";
  import { catalog, itemDialog, links, online, screen, shell, sync, syncNow, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  const paletteState = palette.state;
  // サイドバーのマイタスクのバッジのために 1 回読む(以後は reloads で読み直す)。
  void myTaskBase.reload();

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

  // C キーで作成ダイアログを開く(02「開き方と初期値」)。/ キーで絞り込み欄に移る(01 キーボード)。
  // どちらも、入力中とダイアログを開いているときは使えない。/ は、絞り込み欄のない画面では何もしない。
  function onkeydown(event: KeyboardEvent) {
    const t = event.target as HTMLElement | null;
    const typing = !!t?.closest("input, textarea, select, [contenteditable='true']");
    const dialogOpen = !!document.querySelector('[aria-modal="true"]');
    const field = document.querySelector<HTMLElement>("[data-filter-input]");
    if (
      field &&
      focusesFilter({
        key: event.key,
        ctrl: event.ctrlKey,
        meta: event.metaKey,
        alt: event.altKey,
        typing,
        dialogOpen,
        hasField: true,
      })
    ) {
      event.preventDefault();
      field.focus();
      return;
    }
    if (event.key.toLowerCase() !== "c" || event.ctrlKey || event.metaKey || event.altKey) return;
    if (typing || dialogOpen) return;
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
  <Screen current={$current} stores={$stores} {alert} ondrawer={shell.openDrawer} />
  <DetailPanel />
</AppFrame>
<CreateProjectDialog />
<CreateWorkspaceDialog />
<CreateItemDialog />
<DeleteItemDialog />
<BulkChangeDialog />
<BulkDeleteDialog />
<MoveStatusDialog />
<MoveMenu />
<ChangeKindDialog />
<TransferDialog />
<ItemMenuHost />
<DeleteProjectDialog />
<ProjectTransferDialog />
<ProjectMenu />
<RenameProjectDialog />
<DeleteWorkspaceDialog />
<WorkspaceTransferDialog />
<DiscardDialog />
<!-- Ctrl+K(Cmd+K)で開くコマンドパレット。キーは部品が受ける(02「コマンドパレット」) -->
<CommandPalette
  view={$paletteState}
  mac={isMac()}
  onopen={() => void palette.open()}
  oninput={palette.input}
  onmove={palette.move}
  onchoose={(index, newTab) => void palette.choose(index, newTab)}
  onclose={palette.close}
/>
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
