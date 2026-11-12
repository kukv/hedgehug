<script lang="ts">
  import { childKindOf, type Item, type ProjectView } from "@hedgehug/core";
  import { SvelteSet } from "svelte/reactivity";
  import { itemDialog } from "../../wire";
  import TreeRow from "./TreeRow.svelte";

  // プロジェクトの「ツリー」のタブ。Epic を一番上の階層に並べる(04「ツリー」の「行」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  // 開閉は、画面を出しているあいだだけ覚える。初めはすべて開いている。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
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
