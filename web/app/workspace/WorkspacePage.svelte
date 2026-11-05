<script lang="ts">
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import ListTab from "../list/ListTab.svelte";
  import TreeTab from "../list/TreeTab.svelte";
  import BacklogPage from "../planning/BacklogPage.svelte";
  import { openPlanDialog } from "../planning/dialogs-state.svelte";
  import ReleasesPage from "../planning/ReleasesPage.svelte";
  import SprintsPage from "../planning/SprintsPage.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { router, type WorkspaceTab } from "../router";
  import Empty from "../states/Empty.svelte";
  import StoreBands from "../states/StoreBands.svelte";
  import { useSubscribe } from "../svelte-store";
  import { NO_CANCELED } from "../tree/filter";
  import Button from "../ui/Button.svelte";
  import WorkspaceHeader from "./WorkspaceHeader.svelte";
  import WorkspaceOverview from "./WorkspaceOverview.svelte";
  import WorkspaceSettings from "./WorkspaceSettings.svelte";

  type Props = { store: string; ulid: string; tab: WorkspaceTab };
  let { store, ulid, tab }: Props = $props();

  const c = core();
  const found = useSubscribe(c.stores, () => ({
    entry: c.stores.workspace(store, ulid),
    loaded: c.stores.loaded(),
  }));
  const entry = $derived(found.current.entry);
  // The filter of the tree, the kanban and the table: one for the page, so it stays when the tab
  // changes (it is not saved: D7). The default leaves the canceled tasks out.
  let query = $state(NO_CANCELED);
  // Spread, because Biome reads a prop called scope as the HTML attribute.
  const notFoundProps = $derived({ scope: "workspace" as const, name: ulid });
  const offSentence = useReadOnlySentence(c.stores, () => store);
  const off = $derived(offSentence.current);
</script>

{#if entry}
  <div class="page">
    <StoreBands {store} spaced />
    {#if tab === "overview"}
      <!-- The overview draws its own heading, which has the buttons that make a task or a project. -->
      <WorkspaceOverview {entry} />
    {:else}
      <WorkspaceHeader {entry} {tab}>
        {#snippet actions()}
          {#if tab === "backlog" || tab === "sprints"}
            <Button
              size="sm"
              disabled={off !== null}
              why={off ?? undefined}
              onclick={() => openPlanDialog({ kind: "sprint-add", entry })}
            >
              <Icon name="plus" size={14} />スプリント
            </Button>
          {/if}
          {#if tab === "backlog"}
            <Button
              size="sm"
              disabled={off !== null}
              why={off ?? undefined}
              onclick={() => openCreate({ kind: "task" })}
            >
              <Icon name="plus" size={14} />タスク
            </Button>
          {/if}
          {#if tab === "releases"}
            <Button
              size="sm"
              disabled={off !== null}
              why={off ?? undefined}
              onclick={() => openPlanDialog({ kind: "release-add", entry })}
            >
              <Icon name="plus" size={14} />リリース
            </Button>
          {/if}
        {/snippet}
      </WorkspaceHeader>
      <div class="body">
        {#if tab === "tree"}
          <TreeTab view={{ kind: "workspace", store, ulid }} {query} onquery={(q) => (query = q)} />
        {:else if tab === "kanban" || tab === "table"}
          <ListTab
            view={{ kind: "workspace", store, ulid }}
            layout={tab}
            {query}
            onquery={(q) => (query = q)}
          />
        {:else if tab === "backlog"}
          <BacklogPage {entry} />
        {:else if tab === "sprints"}
          <SprintsPage {entry} />
        {:else if tab === "releases"}
          <ReleasesPage {entry} />
        {:else}
          <WorkspaceSettings {entry} />
        {/if}
      </div>
    {/if}
  </div>
{:else if found.current.loaded}
  <Empty
    variant="notfound"
    {...notFoundProps}
    onHome={() => router.go(router.href({ name: "home" }))}
  />
{:else}
  <p class="note">読み込み中…</p>
{/if}

<style>
  /* The tree scrolls in its own box, so the page is the height of the window and the body takes
     what the heading leaves. */
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .body {
    flex: 1 1 auto;
    min-height: 0;
  }
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
</style>
