<script lang="ts">
  import {
    childKindOf,
    type Item,
    itemDisplayId,
    type ProjectView,
    type Route,
  } from "@hedgehug/core";
  import { SvelteSet } from "svelte/reactivity";
  import { detail, itemDialog, links, projectScreen } from "../../wire";
  import TreeRow from "./TreeRow.svelte";

  // プロジェクトの「ツリー」のタブ。Epic を一番上の階層に並べる(04「ツリー」の「行」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  // 開閉は、画面を出しているあいだだけ覚える。初めはすべて開いている。
  // 期限の判定に使う今日は、画面を出したときの日付(決定 22)。
  const today = new Date();
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
  const route = (it: Item): Route => ({
    name: "item",
    store: view.store.name,
    item: itemDisplayId(view.workspace.prefix, it.number),
  });
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
</script>

<ul class="tree" aria-label="ツリー">
  {#each view.tree as node (node.item.id)}
    <TreeRow
      {node}
      prefix={view.workspace.prefix}
      depth={0}
      {collapsed}
      readOnly={view.store.readOnly}
      ontoggle={toggle}
      onadd={add}
      onopen={open}
      {href}
      settings={view.settings}
      members={view.members}
      items={view.items}
      {today}
      onstatus={(it, id) => void projectScreen.setStatus(it.id, id)}
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
</style>
