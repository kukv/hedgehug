<script lang="ts">
  import {
    childKindOf,
    type Item,
    itemDisplayId,
    menuAt,
    type ProjectView,
    type Route,
  } from "@hedgehug/core";
  import { detail, itemDialog, links, projectScreen } from "../../wire";
  import { itemMenuHost } from "../../wireActions";
  import TreeRow from "./TreeRow.svelte";
  import { createTreeKeyboard } from "./treeKeyboard";
  import { treeTabStops } from "./treeTabStops";

  // プロジェクトの「ツリー」のタブ。Epic を一番上の階層に並べる(04「ツリー」の「行」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  // 期限の判定に使う今日は、画面を出したときの日付(決定 22)。
  const today = new Date();
  let list = $state<HTMLElement>();
  const route = (it: Item): Route => ({
    name: "item",
    store: view.store.name,
    item: itemDisplayId(view.workspace.prefix, it.number),
  });
  // 開閉と、キーのフォーカスのある行は、キー操作の部品が持つ(画面を出しているあいだだけ覚える。初めはすべて開いている)。
  const keyboard = createTreeKeyboard({
    view: () => view,
    list: () => list,
    routeOf: route,
    openMenu: (it, rect) => openMenu(it, rect),
  });
  const focus = keyboard.focus.state;
  // ツリーが変わったとき(読み直し・削除・移動)は、見えている行とフォーカスのある行を直す。描く前に済ませる。
  $effect.pre(() => keyboard.focus.setTree(view.tree));
  const href = (it: Item) => links.href(route(it));
  const open = (e: MouseEvent, it: Item) =>
    links.click(e, route(it), () => void detail.open(view.store.id, it.id));
  function add(parent: Item) {
    const kind = childKindOf(parent.kind);
    if (!kind) return;
    void itemDialog.open({
      workspaceId: parent.workspaceId,
      projectId: parent.projectId,
      kind,
      parentId: parent.id,
    });
  }
  // openMenu は、行のアイテムのメニューを開く。見えている兄弟は、いま見えている行(閉じている行の下は入らない)から決める。
  // rect は、メニューを出す基準のボタン・行の getBoundingClientRect()(行の「⋯」とメニューキーが同じ関数を使う)。
  function openMenu(it: Item, rect: { left: number; bottom: number }) {
    const { x, y } = menuAt(rect);
    const rows = $focus.rows;
    const children = view.items.filter((c) => c.parentId === it.id);
    itemMenuHost.open(it, { rows, children, row: true }, x, y);
  }
</script>

<!-- ツリーは Tab で入る 1 つのフォーカス先で、中の行は矢印で動かす(04「ツリーのキー操作」) -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<ul
  class="tree"
  aria-label="ツリー"
  bind:this={list}
  use:treeTabStops
  onkeydown={keyboard.onkeydown}
  onfocus={keyboard.onfocus}
  onfocusout={keyboard.onfocusout}
  onpointerdown={keyboard.onpointerdown}
>
  {#each view.tree as node (node.item.id)}
    <TreeRow
      {node}
      prefix={view.workspace.prefix}
      depth={0}
      collapsed={$focus.collapsed}
      focusedId={$focus.currentId}
      readOnly={view.store.readOnly}
      ontoggle={keyboard.focus.toggle}
      onadd={add}
      onopen={open}
      {href}
      settings={view.settings}
      members={view.members}
      items={view.items}
      {today}
      onstatus={(it, id) => void projectScreen.setStatus(it.id, id)}
      onmenu={(e, it) => openMenu(it, (e.currentTarget as HTMLElement).getBoundingClientRect())}
    />
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  /* フォーカスのある行は、行全体を囲む枠で示す(行の選択ではない。ツリーにフォーカスがあるあいだだけ) */
  .tree:focus-within :global(.tree-row.focused) {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
</style>
