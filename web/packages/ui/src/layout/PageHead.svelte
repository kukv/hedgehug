<script lang="ts">
  import type { Snippet } from "svelte";
  import IconButton from "../primitives/IconButton.svelte";

  interface Props {
    title: string;
    ondrawer: () => void;
    crumbs?: Snippet;
    actions?: Snippet;
  }

  let { title, ondrawer, crumbs, actions }: Props = $props();
</script>

<header class="page-head">
  <span class="drawer-btn">
    <IconButton label="メニュー" onclick={ondrawer}>
      <svg class="ico" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </IconButton>
  </span>
  <div class="head-main">
    {#if crumbs}
      <div class="crumbs">{@render crumbs()}</div>
    {/if}
    <h1 class="title">{title}</h1>
  </div>
  {#if actions}
    <div class="head-actions">{@render actions()}</div>
  {/if}
</header>

<style>
  .page-head {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 18px max(24px, env(safe-area-inset-right, 0px)) 10px 24px;
  }
  .head-main {
    flex: 1;
    min-width: 0;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    font-size: 12px;
    flex-wrap: wrap;
  }
  .title {
    margin: 2px 0 0;
    font-size: 20px;
    font-weight: 600;
    letter-spacing: 0.005em;
    line-height: 1.35;
    text-wrap: balance;
  }
  .head-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .drawer-btn {
    display: none;
  }
  .ico {
    stroke: currentColor;
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  @container frame (max-width: 860px) {
    .page-head {
      padding: 12px max(16px, env(safe-area-inset-right, 0px)) 8px
        max(16px, env(safe-area-inset-left, 0px));
      flex-wrap: wrap;
    }
    .head-actions {
      width: 100%;
      justify-content: flex-start;
    }
    .drawer-btn {
      display: inline-grid;
      margin-left: -6px;
    }
  }
</style>
