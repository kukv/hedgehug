<script lang="ts">
  import { Input, Menu } from "@hedgehug/ui";
  import { itemMove } from "../../wireActions";

  // 「移動…」の候補のメニュー(02「同じワークスペースの中での移動」)。検索欄を付け、各候補にプロジェクト名を添える。
  // 開くのは itemMove.open(item, x, y)。読み込み・絞り込み・移動・トーストは core の itemMove が持つ。
  const menu = itemMove.state;

  // アイテムの候補だけプロジェクト名を添える(Epic の移し先のプロジェクトは、名前がタイトルそのもの)。
  const items = $derived(
    ($menu?.shown ?? []).map((c) => ({
      id: c.id,
      label: `${c.display} ${c.title}`,
      hint: c.kind === "item" ? c.projectTitle : undefined,
    })),
  );

  // 候補の ID から、押された候補を引く(プロジェクトとアイテムの内部 ID は重ならない)。
  function select(id: string) {
    const c = $menu?.shown.find((x) => x.id === id);
    if (c) void itemMove.choose(c);
  }
</script>

{#if $menu}
  <Menu open x={$menu.x} y={$menu.y} {items} onselect={select} onclose={itemMove.close}>
    {#snippet head()}
      <Input
        label="移動先を検索"
        placeholder="ID かタイトルで検索"
        size="sm"
        focusOnMount
        value={$menu?.query ?? ""}
        oninput={itemMove.setQuery}
      />
    {/snippet}
  </Menu>
{/if}
