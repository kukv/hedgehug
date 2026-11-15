<script lang="ts">
  import { type Item, projectProgress, rowKey, type SearchTreeView } from "@hedgehug/core";
  import { StoreBadge } from "@hedgehug/ui";
  import { openProjectMenu } from "../../projectRows";
  import { addChild, addEpic, itemNodes, itemRoute, projectRoute } from "../../searchTree";
  import { detail, links } from "../../wire";
  import TreeRow from "../project/TreeRow.svelte";
  import ProjectRow from "./ProjectRow.svelte";

  // 検索のツリーの中身(04「ツリー」、E3a-4・12・18)。ワークスペースの見出し(フォーカスの対象にしない)、
  // プロジェクトの行、その下のアイテムの行を並べる。行の鍵は `{storeId}/{内部 ID}`(E3a-5)。
  interface Props {
    view: SearchTreeView;
    collapsed: ReadonlySet<string>;
    focusedId: string | null;
    // dim は、薄く出す行(一致しない祖先)の鍵。
    dim: ReadonlySet<string>;
    ontoggle: (key: string) => void;
    onstatus: (item: Item, statusId: string) => void;
    // onmenu は、アイテムの行の「⋯」。key は行の鍵、rect は「⋯」の位置。
    onmenu: (key: string, rect: DOMRect) => void;
  }

  let { view, collapsed, focusedId, dim, ontoggle, onstatus, onmenu }: Props = $props();

  // 期限の判定に使う今日は、画面を出したときの日付(プロジェクトの画面のツリーと同じ)。
  const today = new Date();
  const rectOf = (e: MouseEvent) => (e.currentTarget as HTMLElement).getBoundingClientRect();
</script>

{#each view.sections as sec (`${sec.store.id}/${sec.workspace.id}`)}
  <li class="ws-head">
    <StoreBadge name={sec.store.name} color={sec.store.color} />
    <span class="ws-name">{sec.workspace.name}</span>
  </li>
  {#each sec.projects as p (p.node.item.id)}
    {@const key = p.node.item.id}
    {@const route = projectRoute(sec.store, p.project)}
    <ProjectRow
      project={p.project}
      store={sec.store}
      progress={projectProgress(p.data.items, p.data.settings)}
      members={p.data.members}
      open={!collapsed.has(key)}
      hasChildren={p.node.children.length > 0}
      focused={focusedId === key}
      rowKey={key}
      ontoggle={() => ontoggle(key)}
      onopen={(e) => links.click(e, route)}
      href={links.href(route)}
      onadd={() => addEpic(p.project)}
      onmenu={(e) => openProjectMenu(sec.store, sec.workspace, p.project, rectOf(e))}
    />
    {#if !collapsed.has(key)}
      {#each itemNodes(p.node.children) as node (rowKey(sec.store.id, node.item.id))}
        <TreeRow
          {node}
          prefix={sec.workspace.prefix}
          depth={1}
          {collapsed}
          {focusedId}
          readOnly={sec.store.readOnly}
          {ontoggle}
          onadd={addChild}
          onopen={(e, it) =>
            links.click(
              e,
              itemRoute(sec.store, sec.workspace, it),
              () => void detail.open(sec.store.id, it.id),
            )}
          href={(it) => links.href(itemRoute(sec.store, sec.workspace, it))}
          settings={p.data.settings}
          members={p.data.members}
          items={p.data.items}
          {today}
          {onstatus}
          onmenu={(e, it) => onmenu(rowKey(sec.store.id, it.id), rectOf(e))}
          rowKey={(it) => rowKey(sec.store.id, it.id)}
          {dim}
          storeColor={sec.store.color}
        />
      {/each}
    {/if}
  {/each}
{/each}

<style>
  .ws-head {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 8px 6px;
    border-bottom: 1px solid var(--line);
  }
  .ws-name {
    font-weight: 500;
    font-size: 13px;
  }
</style>
