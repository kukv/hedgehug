<script lang="ts">
  import {
    activeLabel,
    dayOf,
    overviewFirst,
    readOnlyBand,
    readOnlyNote,
    type SprintCard,
    type SprintDialogKind,
    type StoreNode,
    type WorkspaceNode,
    type WorkspaceTab,
    workspaceTabOf,
  } from "@hedgehug/core";
  import { Button, PageHead, ReadOnlyBand, StoreBadge, Tabs } from "@hedgehug/ui";
  import { navigation, planning, projectDialog, syncNow } from "../../wire";
  import ReleasesTab from "../planning/ReleasesTab.svelte";
  import SprintDialogs from "../planning/SprintDialogs.svelte";
  import SprintsTab from "../planning/SprintsTab.svelte";
  import ReportsTab from "../reports/ReportsTab.svelte";
  import SettingsTab from "../workspace-settings/SettingsTab.svelte";

  // ワークスペースの画面(A2 の計画の持ち場)。タブは「概要」「スプリント」「リリース」「レポート」「設定」。
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
    { id: "reports", label: "レポート" },
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
  const ov = $derived(
    $overview && $overview.storeId === sid && $overview.workspaceId === wid
      ? $overview.overview
      : null,
  );
  const active = $derived(ov ? activeLabel(ov, $syncNow.getFullYear()) : null);

  // スプリントのダイアログ(05)。開いたワークスペースのものだけ出す(別のワークスペースへ移ったら出さない)。
  // 作成・完了・削除は概要(既定・移動先・保持日数)を使うので、読み直してから出す(overviewFirst)。
  // 読み直しても読めなければ出さない(概要の読み込みは断りを出さない。A2 の決定 29)。
  let dialog = $state<{ kind: SprintDialogKind; card: SprintCard | null; wid: string } | null>(
    null,
  );
  async function openSprint(kind: SprintDialogKind, card: SprintCard | null = null) {
    const opening = wid;
    const needs = kind !== "edit" && kind !== "start";
    if (overviewFirst(kind, ov !== null)) await planning.reload();
    if (opening === wid && (!needs || ov)) dialog = { kind, card, wid: opening };
  }
  const closeSprint = () => (dialog = null);
  const shown = $derived(dialog && dialog.wid === wid && tab === "sprints" ? dialog : null);
  // タブやワークスペースを離れたら閉じる(戻ったときに古いカードで出し直さない)。
  $effect(() => {
    if (dialog && !shown) dialog = null;
  });
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
    <SprintsTab
      storeId={store.id}
      storeName={store.name}
      workspaceId={workspace.id}
      readOnly={store.readOnly}
      oncreate={() => void openSprint("create")}
      onstart={(c) => void openSprint("start", c)}
      onclose={(c) => void openSprint("close", c)}
      onedit={(c) => void openSprint("edit", c)}
      ondelete={(c) => void openSprint("delete", c)}
    />
  {:else if tab === "releases"}
    <ReleasesTab
      storeId={store.id}
      storeName={store.name}
      workspaceId={workspace.id}
      readOnly={store.readOnly}
    />
  {:else if tab === "reports"}
    <ReportsTab storeId={store.id} storeName={store.name} workspaceId={workspace.id} />
  {/if}
  {#if shown}
    <SprintDialogs
      storeId={sid}
      workspaceId={wid}
      kind={shown.kind}
      card={shown.card}
      overview={ov}
      today={dayOf($syncNow)}
      onclose={closeSprint}
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
