<script lang="ts">
  import { Menu } from "@hedgehug/ui";
  import { itemMenuHost } from "../../wireActions";
  import SaveAsTemplateDialog from "./SaveAsTemplateDialog.svelte";

  // アイテムのメニュー(02「上から順の構成」、04「ツリー」の「行」)。詳細パネルの「⋯」とツリーの行の「⋯」が
  // 同じメニューを開く(開くのは itemMenuHost.open)。項目・無効の理由・選んだあとの動きは core の itemMenuHost が持つ。
  const menu = itemMenuHost.state;
  const template = itemMenuHost.template;

  // 「削除…」だけ危険な操作の見た目にする。
  const items = $derived(($menu?.entries ?? []).map((e) => ({ ...e, danger: e.id === "delete" })));

  function select(id: string) {
    const entry = $menu?.entries.find((e) => e.id === id);
    if (entry) void itemMenuHost.select(entry.id);
  }
</script>

{#if $menu}
  <Menu open x={$menu.x} y={$menu.y} {items} onselect={select} onclose={itemMenuHost.close} />
{/if}
{#if $template}
  <SaveAsTemplateDialog v={$template} onclose={itemMenuHost.closeTemplate} />
{/if}
