<script lang="ts">
  import { AppFrame, Brand, PageHead } from "@hedgehug/ui";
  import { shell, system } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  system.load();
</script>

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Brand />
  {/snippet}
  <PageHead title="hedgehug" ondrawer={shell.openDrawer} />
  <div class="content">
    {#if $version}
      <p class="version">{$version}</p>
    {/if}
  </div>
</AppFrame>

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
