<script lang="ts">
  import {
    childKindOf,
    type Item,
    itemDisplayId,
    type Member,
    readOnlyNote,
    type TreeNode,
    type WorkspaceSettings,
  } from "@hedgehug/core";
  import { Avatar, IconButton, KindIcon } from "@hedgehug/ui";
  import StatusButton from "../item/StatusButton.svelte";
  import Self from "./TreeRow.svelte";
  import TreeRowMeta from "./TreeRowMeta.svelte";

  // ツリーの 1 行と、その下の行(04「ツリー」の「行」のうち、A2 の計画の決定 17 の分)。
  interface Props {
    node: TreeNode;
    prefix: string;
    depth: number;
    collapsed: ReadonlySet<string>;
    // focusedId は、キーのフォーカスのある行(04「ツリーのキー操作」)。
    focusedId: string | null;
    readOnly: boolean;
    ontoggle: (id: string) => void;
    // onadd は、行の「+」(その行を親にした作成ダイアログ)。
    onadd: (parent: Item) => void;
    // onopen は、タイトルを押したとき(詳細パネルを開く)。Ctrl+クリックなどは新しいタブでアイテムの入口を開く。
    onopen: (event: MouseEvent, item: Item) => void;
    href: (item: Item) => string;
    settings: WorkspaceSettings;
    members: Member[];
    items: Item[];
    today: Date;
    onstatus: (item: Item, statusId: string) => void;
    // onmenu は、行の「⋯」(アイテムのメニュー)。
    onmenu: (event: MouseEvent, item: Item) => void;
  }

  let {
    node,
    prefix,
    depth,
    collapsed,
    focusedId,
    readOnly,
    ontoggle,
    onadd,
    onopen,
    href,
    settings,
    members,
    items,
    today,
    onstatus,
    onmenu,
  }: Props = $props();

  const it = $derived(node.item);
  const open = $derived(!collapsed.has(it.id));
  const child = $derived(childKindOf(it.kind));
  const member = $derived(members.find((x) => x.login.toLowerCase() === it.assignee.toLowerCase()));
</script>

<li
  class="tree-row"
  class:focused={focusedId === it.id}
  data-row={it.id}
  aria-current={focusedId === it.id ? "true" : undefined}
  style:--d={depth}
>
  {#if node.children.length > 0}
    <button
      type="button"
      class="twisty"
      class:open
      aria-label="開閉"
      aria-expanded={open}
      onclick={() => ontoggle(it.id)}
    >
      ›
    </button>
  {:else}
    <span class="twisty-space"></span>
  {/if}
  <KindIcon kind={it.kind} />
  <code class="id">{itemDisplayId(prefix, it.number)}</code>
  <a
    class="row-title"
    href={href(it)}
    onclick={(e) => onopen(e, it)}
    onauxclick={(e) => onopen(e, it)}
    >{it.title}</a
  >
  <TreeRowMeta {it} {items} {settings} {today} />
  <StatusButton
    status={it.status}
    {settings}
    disabled={readOnly}
    onchange={(id) => onstatus(it, id)}
  />
  {#if it.assignee}
    <Avatar name={member?.name ?? ""} title={member?.name ?? "不明なユーザー"} />
  {/if}
  {#if child}
    <IconButton
      label="{child} を追加"
      size="xs"
      disabled={readOnly}
      title={readOnly ? readOnlyNote : undefined}
      onclick={() => onadd(it)}
      >+</IconButton
    >
  {/if}
  <IconButton
    label="{itemDisplayId(prefix, it.number)} のメニュー"
    size="xs"
    onclick={(e) => onmenu(e, it)}
    >⋯</IconButton
  >
</li>
{#if open}
  {#each node.children as c (c.item.id)}
    <Self
      node={c}
      {prefix}
      depth={depth + 1}
      {collapsed}
      {focusedId}
      {readOnly}
      {ontoggle}
      {onadd}
      {onopen}
      {href}
      {settings}
      {members}
      {items}
      {today}
      {onstatus}
      {onmenu}
    />
  {/each}
{/if}

<style>
  .tree-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 8px 2px calc(8px + var(--d) * 22px);
    border-bottom: 1px solid var(--line);
  }
  .twisty,
  .twisty-space {
    width: 18px;
    height: 18px;
    flex: none;
  }
  .twisty {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
