<script lang="ts">
  import {
    type ProjectTab,
    projectCategory,
    type Route,
    readOnlyBand,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, PageHead, ReadOnlyBand, StatusDot, StoreBadge, Tabs } from "@hedgehug/ui";
  import { itemDialog, links, navigation, projectScreen } from "../../wire";
  import ProjectOverview from "./ProjectOverview.svelte";
  import ProjectTree from "./ProjectTree.svelte";

  // プロジェクトの画面(02「画面の見出しと構成」)。タブは「概要」「ツリー」だけ(A2 の計画の決定 15)。
  interface Props {
    storeId: string;
    projectId: string;
    tab: ProjectTab;
    ondrawer: () => void;
    alert: boolean;
  }

  let { storeId, projectId, tab, ondrawer, alert }: Props = $props();

  const view = projectScreen.view;
  $effect(() => {
    void projectScreen.load(storeId, projectId);
  });

  const tabs = [
    { id: "overview", label: "概要" },
    { id: "tree", label: "ツリー" },
  ];
  const wsRoute = $derived<Route | null>(
    $view ? { name: "workspace", store: $view.store.name, prefix: $view.workspace.prefix } : null,
  );
  function select(id: string) {
    if (!$view) return;
    const next = id === "tree" ? "tree" : "overview";
    navigation.go({
      name: "project",
      store: $view.store.name,
      project: $view.project.displayId,
      tab: next,
    });
  }
  // addEpic は、このプロジェクトを親にした Epic の作成ダイアログを開く(02「画面の見出しと構成」)。
  function addEpic() {
    if (!$view) return;
    void itemDialog.open({
      workspaceId: $view.workspace.id,
      projectId: $view.project.id,
      kind: "Epic",
    });
  }
</script>

{#if $view && $view.store.id === storeId && $view.project.id === projectId && wsRoute}
  {@const p = $view.project}
  <PageHead title="{p.displayId} {p.title}" {ondrawer} {alert}>
    {#snippet crumbs()}
      <StoreBadge name={$view.store.name} color={$view.store.color} />
      <a
        class="crumb"
        href={links.href(wsRoute)}
        onclick={(e) => links.click(e, wsRoute)}
        onauxclick={(e) => links.click(e, wsRoute)}
        >{$view.workspace.name}</a
      >
    {/snippet}
    {#snippet actions()}
      <span class="status-pill">
        <StatusDot category={projectCategory(p.status)} label={p.status} />{p.status}
      </span>
      <Button
        variant="primary"
        disabled={$view.store.readOnly}
        title={$view.store.readOnly ? readOnlyNote : undefined}
        onclick={addEpic}
        ><span aria-hidden="true">+</span>
        Epic</Button
      >
    {/snippet}
  </PageHead>
  <Tabs {tabs} current={tab} onselect={select} />
  <div class="content">
    {#if $view.store.readOnly}
      <ReadOnlyBand text={readOnlyBand($view.store.name)} />
    {/if}
    {#if tab === "tree"}
      <ProjectTree view={$view} />
    {:else}
      <ProjectOverview view={$view} onaddepic={addEpic} />
    {/if}
  </div>
{/if}

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .crumb {
    color: var(--muted);
    text-decoration: none;
  }
  .crumb:hover {
    color: var(--ink);
    text-decoration: underline;
  }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 10px;
    border: 1px solid var(--line);
    border-radius: 999px;
    font-size: 12.5px;
  }
</style>
