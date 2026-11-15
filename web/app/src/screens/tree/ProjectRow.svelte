<script lang="ts">
  import {
    type Member,
    ownerName,
    type ProjectNode,
    progressPercent,
    projectCategory,
    readOnlyNote,
    type StoreNode,
  } from "@hedgehug/core";
  import { IconButton, KindIcon, ProgressBar, StatusDot } from "@hedgehug/ui";

  // ツリーのプロジェクトの行(04「ツリー」の「行」、E3a-12)。プロジェクトの下の行は、置く側が続けて並べる。
  interface Props {
    project: ProjectNode;
    store: StoreNode;
    // progress は、「完了/対象」(core の projectProgress)。
    progress: { done: number; total: number };
    members: Member[];
    open: boolean;
    hasChildren: boolean;
    focused: boolean;
    // rowKey は、行の鍵({storeId}/{内部 ID}。E3a-5)。
    rowKey: string;
    ontoggle: () => void;
    // onopen は、タイトルを押したとき(プロジェクトの概要へ。Ctrl+クリックなどは新しいタブ)。
    onopen: (event: MouseEvent) => void;
    href: string;
    // onadd は、行の「+」(その下に Epic を作る作成ダイアログ)。
    onadd: () => void;
    onmenu: (event: MouseEvent) => void;
  }

  let {
    project,
    store,
    progress,
    members,
    open,
    hasChildren,
    focused,
    rowKey,
    ontoggle,
    onopen,
    href,
    onadd,
    onmenu,
  }: Props = $props();
</script>

<li class="tree-row" class:focused data-row={rowKey} aria-current={focused ? "true" : undefined}>
  {#if hasChildren}
    <button
      type="button"
      class="twisty"
      class:open
      aria-label="開閉"
      aria-expanded={open}
      onclick={ontoggle}
    >
      ›
    </button>
  {:else}
    <span class="twisty-space"></span>
  {/if}
  <i
    class="store-dot"
    role="img"
    aria-label={store.name}
    title={store.name}
    style="--c: var(--t-{store.color}-fg)"
  ></i>
  <KindIcon kind="Project" />
  <a class="row-title" {href} onclick={onopen} onauxclick={onopen}>{project.title}</a>
  <span class="status">
    <StatusDot category={projectCategory(project.status)} label={project.status} />{project.status}
  </span>
  <span class="owner">{ownerName(project.owner, members)}</span>
  <ProgressBar percent={progressPercent(progress)} detail="{progress.done} / {progress.total}" />
  <IconButton
    label="Epic を追加"
    size="xs"
    disabled={store.readOnly}
    title={store.readOnly ? readOnlyNote : undefined}
    onclick={onadd}
    >+</IconButton
  >
  <IconButton label="{project.title} のメニュー" size="xs" onclick={onmenu}>⋯</IconButton>
</li>

<style>
  .tree-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 8px;
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
  .store-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--c);
    flex: none;
  }
  .row-title {
    color: var(--ink);
    font-weight: 500;
    text-decoration: none;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .status,
  .owner {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
