<script lang="ts">
  import { onMount } from "svelte";
  import { takeToken } from "../core/token";
  import { auth } from "./auth-state.svelte";
  import { setCore } from "./context";
  import { createCore } from "./core-factory";
  import { router } from "./router";
  import Sidebar from "./sidebar/Sidebar.svelte";
  import Banners from "./states/Banners.svelte";
  import Disconnected from "./states/Disconnected.svelte";
  import NoToken from "./states/NoToken.svelte";
  import Onboarding from "./states/Onboarding.svelte";
  import { useSubscribe } from "./svelte-store";
  import TreeView from "./tree/TreeView.svelte";
  import Toasts from "./ui/Toasts.svelte";

  const token = takeToken(location, sessionStorage, history);
  const c = createCore({ token: () => token, onUnauthorized: () => auth.markLost() });
  setCore(c);

  const conn = useSubscribe(c.conn, () => ({ state: c.conn.state, lastOpenAt: c.conn.lastOpenAt }));
  const dir = useSubscribe(c.stores, () => {
    const first = c.stores.workspaces()[0];
    return {
      loaded: c.stores.loaded(),
      count: c.stores.stores().length,
      first: first ? { store: first.store, ulid: first.ulid } : null,
    };
  });
  const noToken = $derived(token === null || auth.lost);
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

<div class="app" inert={blocked}>
  <aside class="side">
    <Sidebar />
  </aside>
  <div class="body">
    <Banners />
    <main class="main" id="main">
      <!-- routes -->
      {#if router.route.name === "workspace"}
        {#key `${router.route.store}/${router.route.ulid}`}
          <TreeView
            view={{ kind: "workspace", store: router.route.store, ulid: router.route.ulid }}
          />
        {/key}
      {:else if router.route.name === "project"}
        {#key router.route.ulid}
          <TreeView view={{ kind: "project", ulid: router.route.ulid }} />
        {/key}
      {:else if router.route.name === "home" && dir.current.first}
        <!-- "/" is the tree of the first workspace -->
        {#key `${dir.current.first.store}/${dir.current.first.ulid}`}
          <TreeView view={{ kind: "workspace", ...dir.current.first }} />
        {/key}
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
