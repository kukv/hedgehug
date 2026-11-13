<script lang="ts">
  import { type DetailView, menuAt } from "@hedgehug/core";
  import { IconButton } from "@hedgehug/ui";
  import { itemMenuHost } from "../../wireActions";

  // 詳細パネルのメニューの「⋯」(02「上から順の構成」)。メニューの本体は App.svelte に 1 つだけ置いた
  // ItemMenuHost.svelte。ここは、開く位置と、見えている兄弟(詳細パネルは絞り込みがないので全兄弟)を渡すだけ。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  function show(event: MouseEvent) {
    const { x, y } = menuAt((event.currentTarget as HTMLElement).getBoundingClientRect());
    itemMenuHost.open(v.item, { rows: v.siblings, children: v.children, row: false }, x, y);
  }
</script>

<IconButton label="メニュー" size="sm" onclick={show}>⋯</IconButton>
