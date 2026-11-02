<script lang="ts">
  import { onMount } from "svelte";
  import { cliWorkspaceAdd, Placeholder } from "../core/cli";
  import { takeToken } from "../core/token";
  import { setCore } from "./context";
  import { createCore } from "./core-factory";
  import CreateHost from "./create/CreateHost.svelte";
  import TaskDetail from "./detail/TaskDetail.svelte";
  import CommandPalette from "./palette/CommandPalette.svelte";
  import { router } from "./router";
  import SettingsRoute from "./settings/SettingsRoute.svelte";
  import Sidebar from "./sidebar/Sidebar.svelte";
  import Banners from "./states/Banners.svelte";
  import Disconnected from "./states/Disconnected.svelte";
  import Empty from "./states/Empty.svelte";
  import NoToken from "./states/NoToken.svelte";
  import Onboarding from "./states/Onboarding.svelte";
  import { useSubscribe } from "./svelte-store";
  import TreeView from "./tree/TreeView.svelte";
  import Toasts from "./ui/Toasts.svelte";
  import WorkspaceDialogs from "./workspace/WorkspaceDialogs.svelte";
  import WorkspacePage from "./workspace/WorkspacePage.svelte";

  const token = takeToken(location, sessionStorage, history);
  // Whether the server refused the token (401 auth.token); this page then shows NoToken.
  let lost = $state(false);
  const c = createCore({ token: () => token, onUnauthorized: () => (lost = true) });
  setCore(c);

  const conn = useSubscribe(c.conn, () => ({ state: c.conn.state, lastOpenAt: c.conn.lastOpenAt }));
  const dir = useSubscribe(c.stores, () => {
    const first = c.stores.workspaces()[0];
    return {
      loaded: c.stores.loaded(),
      count: c.stores.stores().length,
      names: c.stores.stores().map((s) => s.name),
      first: first ? { store: first.store, ulid: first.ulid } : null,
    };
  });
  const route = $derived(router.route);
  // The settings take the whole window: their nav stands where the sidebar stands. The sidebar
  // stays mounted, because the keys of the app (⌘K, ⇧⌘S) are registered by it.
  const inSettings = $derived(route.name === "settings");
  const noToken = $derived(token === null || lost);
  const onboarding = $derived(dir.current.loaded && dir.current.count === 0);
  const down = $derived(!noToken && conn.current.state === "down");
  // The gates cover the screen; nothing behind them may take focus or keys.
  const blocked = $derived(noToken || onboarding || down);

  // A refused token will not get better by trying again.
  $effect(() => {
    if (noToken) c.conn.stop();
  });

  onMount(() => {
    router.go(location.pathname, { replace: true });
    if (token) c.conn.start();
    return () => c.conn.stop();
  });
</script>

<div class="app" class:settings={inSettings} inert={blocked}>
  <aside class="side" hidden={inSettings}>
    <Sidebar />
  </aside>
  <div class="body">
    <Banners />
    <main class="main" class:flush={inSettings} id="main">
      <!-- routes -->
      {#if route.name === "workspace"}
        {#key `${route.store}/${route.ulid}`}
          <WorkspacePage store={route.store} ulid={route.ulid} tab={route.tab} />
        {/key}
      {:else if route.name === "project"}
        {#key route.ulid}
          <TreeView view={{ kind: "project", ulid: route.ulid }} />
        {/key}
      {:else if route.name === "task"}
        {#key route.ulid}
          <TaskDetail ulid={route.ulid} />
        {/key}
      {:else if route.name === "settings"}
        <SettingsRoute page={route} />
      {:else if route.name === "home" && dir.current.first}
        <!-- "/" is the overview of the first workspace -->
        {#key `${dir.current.first.store}/${dir.current.first.ulid}`}
          <WorkspacePage
            store={dir.current.first.store}
            ulid={dir.current.first.ulid}
            tab="overview"
          />
        {/key}
      {:else if route.name === "home" && dir.current.loaded && dir.current.count > 0}
        <!-- stores exist but none has a workspace -->
        <Empty
          variant="noworkspace"
          commands={dir.current.names.map((store) =>
            cliWorkspaceAdd({
              name: new Placeholder("名前"),
              prefix: new Placeholder("接頭辞"),
              store,
            }),
          )}
        />
      {/if}
    </main>
  </div>
</div>
{#if noToken}
  <NoToken />
{:else if onboarding}
  <Onboarding />
{/if}
{#if down}
  <Disconnected lastOpenAt={conn.current.lastOpenAt} />
{/if}
<CreateHost />
<WorkspaceDialogs />
<CommandPalette />
<Toasts />

<style>
  .app {
    display: grid;
    grid-template-columns: var(--sidebar-w) minmax(0, 1fr);
    /* The window is the height: the tree scrolls in its own box, which a long tree needs to
       draw only the rows in view. The rest of the pages scroll in .main. */
    height: 100vh;
  }
  .side {
    position: sticky;
    top: 0;
    height: 100vh;
    overflow-y: auto;
    border-right: 1px solid var(--line);
    background: var(--sidebar);
  }
  /* The settings have their own nav in place of the sidebar. */
  .app.settings {
    grid-template-columns: minmax(0, 1fr);
  }
  .body {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .main {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    padding: var(--space-20) var(--gutter);
  }
  .main.flush {
    padding: 0;
  }
  @media (max-width: 819px) {
    .app {
      grid-template-columns: minmax(0, 1fr);
      height: auto;
      min-height: 100vh;
    }
    .side {
      position: static;
      height: auto;
      border-right: 0;
      border-bottom: 1px solid var(--line);
    }
  }
</style>
