<script lang="ts">
  import {
    readOnlyBand,
    readOnlyNote,
    type StoreNode,
    type WorkspaceNode,
    type WorkspaceTab,
  } from "@hedgehug/core";
  import { Button, PageHead, ReadOnlyBand, StoreBadge, Tabs } from "@hedgehug/ui";
  import { navigation, projectDialog } from "../../wire";
  import SettingsTab from "../workspace-settings/SettingsTab.svelte";

  // ワークスペースの画面の見出しだけの枠(A2 の計画の持ち場)。タブの中身はトラック B・E・F が足す。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
    tab: WorkspaceTab;
    ondrawer: () => void;
  }

  let { store, workspace, tab, ondrawer }: Props = $props();

  // タブは「概要」「設定」だけ(B2 の計画の決定 15)。履歴に積む(01)。
  const tabs = [
    { id: "overview", label: "概要" },
    { id: "settings", label: "設定" },
  ];
  function select(id: string) {
    navigation.go({
      name: "workspace",
      store: store.name,
      prefix: workspace.prefix,
      tab: id === "settings" ? "settings" : "overview",
    });
  }
</script>

<PageHead title={workspace.name} {ondrawer}>
  {#snippet crumbs()}
    <StoreBadge name={store.name} color={store.color} />
    <code class="prefix">{workspace.prefix}</code>
  {/snippet}
  {#snippet actions()}
    <Button
      disabled={store.readOnly}
      title={store.readOnly ? readOnlyNote : undefined}
      onclick={() => void projectDialog.open(workspace.id)}
      >プロジェクト</Button
    >
  {/snippet}
</PageHead>
<Tabs {tabs} current={tab} onselect={select} />
<div class="content">
  {#if tab === "settings"}
    <SettingsTab storeId={store.id} workspaceId={workspace.id} storeName={store.name} />
  {:else if store.readOnly}
    <ReadOnlyBand text={readOnlyBand(store.name)} />
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
</style>
