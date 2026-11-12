<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { PageHead } from "@hedgehug/ui";
  import ProjectScreen from "../screens/project/ProjectScreen.svelte";
  import WorkspaceScreen from "../screens/workspace/WorkspaceScreen.svelte";

  // いまの画面を出す。画面が替わるたびに作り直すので、本文は先頭から出る(01「画面の移り方」)。
  interface Props {
    current: Resolved | null;
    stores: StoreNode[];
    version: string | null;
    ondrawer: () => void;
  }

  let { current, stores, version, ondrawer }: Props = $props();

  const store = $derived(
    current && current.name !== "home" ? stores.find((s) => s.id === current.storeId) : undefined,
  );
  const workspace = $derived(
    current?.name === "workspace"
      ? store?.workspaces.find((w) => w.id === current.workspaceId)
      : undefined,
  );
</script>

{#key current}
  {#if current?.name === "project"}
    <ProjectScreen
      storeId={current.storeId}
      projectId={current.projectId}
      tab={current.tab}
      {ondrawer}
    />
  {:else if store && workspace}
    <WorkspaceScreen {store} {workspace} {ondrawer} />
  {:else}
    <!-- ホーム。マイタスク(トラック E)が入ったら替える(A2 の計画の決定 6)。 -->
    <PageHead title="hedgehug" {ondrawer} />
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
