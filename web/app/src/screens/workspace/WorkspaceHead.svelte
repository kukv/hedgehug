<script lang="ts">
  import {
    readOnlyNote,
    type StoreNode,
    type WorkspaceNode,
    type WorkspaceTab,
  } from "@hedgehug/core";
  import { Button, PageHead, StoreBadge } from "@hedgehug/ui";
  import { projectDialog, screen } from "../../wire";

  // ワークスペースの画面の見出しと、古いアドレスから開いたときの移動の 1 行(WorkspaceScreen から分けた)。
  // active は見出しの「実行中」(実行中のスプリントがないときは null)。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
    tab: WorkspaceTab;
    active: string | null;
    ondrawer: () => void;
    alert: boolean;
  }

  let { store, workspace, tab, active, ondrawer, alert }: Props = $props();

  // 古いアドレスから開いたときの移動の 1 行は、出しているワークスペースとタブが同じときだけ出す(B5c-2)。
  const workspaceMoved = screen.workspaceMoved;
  const moved = $derived.by(() => {
    const m = $workspaceMoved;
    return m?.storeId === store.id && m.workspaceId === workspace.id && m.tab === tab
      ? m.text
      : null;
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
{#if moved}
  <p class="moved">{moved}</p>
{/if}

<style>
  .moved {
    margin: 0;
    padding: 0 24px 8px;
    font-size: 12.5px;
    color: var(--muted);
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
