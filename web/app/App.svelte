<script lang="ts">
  import { onMount } from "svelte";
  import { takeToken } from "../core/token";
  import { setCore } from "./context";
  import { createCore } from "./core-factory";
  import { router } from "./router";
  import Toasts from "./ui/Toasts.svelte";

  const token = takeToken(location, sessionStorage, history);
  const c = createCore({ token: () => token });
  setCore(c);

  onMount(() => {
    router.go(location.pathname, { replace: true });
    c.conn.start();
    return () => c.conn.stop();
  });
</script>

<div class="app">
  <aside class="side">
    <!-- sidebar -->
  </aside>
  <div class="body">
    <!-- banners -->
    <main class="main" id="main">
      <!-- routes -->
    </main>
  </div>
</div>
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
