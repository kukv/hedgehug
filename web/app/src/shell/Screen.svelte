<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { PageHead } from "@hedgehug/ui";
  import ProjectScreen from "../screens/project/ProjectScreen.svelte";
  import SettingsScreen from "../screens/settings/SettingsScreen.svelte";
  import WorkspaceScreen from "../screens/workspace/WorkspaceScreen.svelte";

  // いまの画面を出す。画面が替わるたびに作り直すので、本文は先頭から出る(01「画面の移り方」)。
  interface Props {
    current: Resolved | null;
    stores: StoreNode[];
    version: string | null;
    alert: boolean;
    ondrawer: () => void;
  }

  let { current, stores, version, alert, ondrawer }: Props = $props();

  const store = $derived(
    current && "storeId" in current ? stores.find((s) => s.id === current.storeId) : undefined,
  );
  const workspace = $derived(
    current?.name === "workspace"
      ? store?.workspaces.find((w) => w.id === current.workspaceId)
      : undefined,
  );
  const workspaceTab = $derived(current?.name === "workspace" ? current.tab : "overview");
</script>

{#key current}
  {#if current?.name === "settings"}
    <SettingsScreen tab={current.tab} {ondrawer} {alert} />
  {:else if current?.name === "project"}
    <ProjectScreen
      storeId={current.storeId}
      projectId={current.projectId}
      tab={current.tab}
      {ondrawer}
      {alert}
    />
  {:else if store && workspace}
    <WorkspaceScreen {store} {workspace} tab={workspaceTab} {ondrawer} {alert} />
  {:else}
    <!-- ホーム。マイタスク(トラック E)が入ったら替える(A2 の計画の決定 6)。 -->
    <PageHead title="hedgehug" {ondrawer} {alert} />
    <div class="content">
      {#if version}
        <p class="version">{version}</p>
      {/if}
    </div>
  {/if}
{/key}

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .version {
    color: var(--muted);
    font-family: var(--font-mono);
    margin: 0;
  }
</style>
