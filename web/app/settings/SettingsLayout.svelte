<script lang="ts">
  import type { Snippet } from "svelte";
  import type { SettingsPage } from "../router";
  import SettingsNav from "./SettingsNav.svelte";

  let { page, children }: { page: SettingsPage; children?: Snippet } = $props();
</script>

<!-- The whole window is one layout: the nav on the left takes the place of the main sidebar. -->
<div class="settings">
  <div class="nav">
    <SettingsNav {page} />
  </div>
  <section class="content" aria-label="設定の本文">
    {@render children?.()}
  </section>
</div>

<style>
  .settings {
    display: grid;
    grid-template-columns: var(--sidebar-w) minmax(0, 1fr);
    height: 100%;
    min-height: 0;
  }
  .nav {
    overflow-y: auto;
    border-right: 1px solid var(--line);
    background: var(--sidebar);
  }
  .content {
    min-width: 0;
    min-height: 0;
    overflow: auto;
    padding: var(--space-20) var(--gutter);
  }
  @media (max-width: 819px) {
    .settings {
      grid-template-columns: minmax(0, 1fr);
      height: auto;
    }
    .nav {
      border-right: 0;
      border-bottom: 1px solid var(--line);
    }
  }
</style>
