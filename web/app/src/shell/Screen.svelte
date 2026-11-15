<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import ActivityScreen from "../screens/activity/ActivityScreen.svelte";
  import InboxScreen from "../screens/inbox/InboxScreen.svelte";
  import MyTasksScreen from "../screens/mytasks/MyTasksScreen.svelte";
  import ProjectScreen from "../screens/project/ProjectScreen.svelte";
  import SearchScreen from "../screens/search/SearchScreen.svelte";
  import SettingsScreen from "../screens/settings/SettingsScreen.svelte";
  import ViewScreen from "../screens/views/ViewScreen.svelte";
  import WorkspaceScreen from "../screens/workspace/WorkspaceScreen.svelte";

  // いまの画面を出す。画面が替わるたびに作り直すので、本文は先頭から出る(01「画面の移り方」)。
  interface Props {
    current: Resolved | null;
    stores: StoreNode[];
    alert: boolean;
    ondrawer: () => void;
  }

  let { current, stores, alert, ondrawer }: Props = $props();

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
  {:else if current?.name === "inbox"}
    <InboxScreen {ondrawer} {alert} />
  {:else if current?.name === "activity"}
    <ActivityScreen {ondrawer} {alert} />
  {:else if current?.name === "mytasks"}
    <MyTasksScreen {ondrawer} {alert} />
  {:else if current?.name === "search"}
    <SearchScreen {ondrawer} {alert} />
  {:else if current?.name === "view"}
    <ViewScreen id={current.viewId} {ondrawer} {alert} />
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
  {/if}
{/key}
