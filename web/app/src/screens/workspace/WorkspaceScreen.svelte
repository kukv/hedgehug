<script lang="ts">
  import {
    activeLabel,
    readOnlyBand,
    readOnlyNote,
    type StoreNode,
    type WorkspaceNode,
    type WorkspaceTab,
    workspaceTabOf,
  } from "@hedgehug/core";
  import { Button, PageHead, ReadOnlyBand, StoreBadge, Tabs } from "@hedgehug/ui";
  import { navigation, planning, projectDialog, syncNow } from "../../wire";
  import SprintsTab from "../planning/SprintsTab.svelte";
  import SettingsTab from "../workspace-settings/SettingsTab.svelte";

  // ワークスペースの画面(A2 の計画の持ち場)。タブは「概要」「スプリント」「リリース」「設定」。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
    tab: WorkspaceTab;
    ondrawer: () => void;
    alert: boolean;
  }

  let { store, workspace, tab, ondrawer, alert }: Props = $props();

  // タブは作ったものだけ(B2 の計画の決定 15、F1-20)。履歴に積む(01)。
  const tabs = [
    { id: "overview", label: "概要" },
    { id: "sprints", label: "スプリント" },
    { id: "releases", label: "リリース" },
    { id: "settings", label: "設定" },
  ];
  function select(id: string) {
    navigation.go({
      name: "workspace",
      store: store.name,
      prefix: workspace.prefix,
      tab: workspaceTabOf(id),
    });
  }

  // 見出しの「実行中」は、実行中のスプリントがあるときだけ出す(05)。Store の出来事で読み直す(wire.ts)。
  // 親が Store の読み直しで作り直した値を渡しても、ID が同じなら開き直さない。
  const overview = planning.state;
  const sid = $derived(store.id);
  const wid = $derived(workspace.id);
  // 画面を閉じたら(別のワークスペースへ移るときも)、前のワークスペースを読み直さない。
  $effect(() => {
    void planning.open(sid, wid);
    return () => planning.close();
  });
  const active = $derived(
    $overview && $overview.storeId === store.id && $overview.workspaceId === workspace.id
      ? activeLabel($overview.overview, $syncNow.getFullYear())
      : null,
  );
</script>

<PageHead title={workspace.name} {ondrawer} {alert}>
  {#snippet crumbs()}
    <StoreBadge name={store.name} color={store.color} />
    <code class="prefix">{workspace.prefix}</code>
    {#if active}
      <span class="running">{active}</span>
    {/if}
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
  {#if tab === "sprints"}
    <!-- 作成・開始・完了・編集・削除のダイアログは Task 16 でつなぐ -->
    <SprintsTab
      storeId={store.id}
      storeName={store.name}
      workspaceId={workspace.id}
      readOnly={store.readOnly}
      oncreate={() => {}}
      onstart={() => {}}
      onclose={() => {}}
      onedit={() => {}}
      ondelete={() => {}}
    />
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
  .running {
    font-size: 12px;
    color: var(--muted);
  }
</style>
