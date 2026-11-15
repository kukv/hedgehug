<script lang="ts">
  import type { ProjectMenuId } from "@hedgehug/core";
  import { Menu } from "@hedgehug/ui";
  import { closeProjectMenu, projectMenuState, selectProjectMenu } from "../../projectRows";

  // ツリーのプロジェクトの行のメニュー(E3a-13)。項目・無効の理由は core の projectMenu、
  // 選んだあとの動きは projectRows.ts が持つ。「削除…」だけ危険な操作の見た目にする。
  const menu = projectMenuState;
  const items = $derived(($menu?.items ?? []).map((i) => ({ ...i, danger: i.id === "delete" })));
</script>

{#if $menu}
  <Menu
    open
    x={$menu.x}
    y={$menu.y}
    {items}
    onselect={(id) => selectProjectMenu(id as ProjectMenuId)}
    onclose={closeProjectMenu}
  />
{/if}
