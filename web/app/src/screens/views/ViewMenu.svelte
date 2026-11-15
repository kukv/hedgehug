<script lang="ts">
  import { deleteViewBody, viewMenuItems } from "@hedgehug/core";
  import { ConfirmDialog, Menu } from "@hedgehug/ui";
  import {
    cancelDeleteView,
    closeViewMenu,
    confirmDeleteView,
    deletingView,
    selectViewMenu,
    viewMenuState,
  } from "../../views";

  // ビューの画面の見出しのメニュー(E3b-13)と、「削除…」の確認。項目・文は core の viewMenu。「削除…」だけ
  // 危険な操作の見た目にする。
  const menu = viewMenuState;
  const deleting = deletingView;
  const items = viewMenuItems.map((i) => ({ ...i, danger: i.id === "delete" }));
</script>

{#if $menu}
  <Menu
    open
    x={$menu.x}
    y={$menu.y}
    {items}
    onselect={(id) => selectViewMenu(id as "rename" | "duplicate" | "delete")}
    onclose={closeViewMenu}
  />
{/if}
{#if $deleting}
  <ConfirmDialog
    title="ビューを削除"
    lines={[deleteViewBody($deleting.name)]}
    ok="削除"
    danger
    onok={() => void confirmDeleteView()}
    oncancel={cancelDeleteView}
  />
{/if}
