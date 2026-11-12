<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    drawerOpen: boolean;
    onclosedrawer: () => void;
    sidebar: Snippet;
    children: Snippet;
  }

  let { drawerOpen, onclosedrawer, sidebar, children }: Props = $props();
</script>

<!-- 枠の幅で形を変える(コンテナクエリ)。860px 以下では、サイドバーを引き出しにする。 -->
<div class="frame">
  <div class="app" class:drawer-open={drawerOpen}>
    <aside class="side">{@render sidebar()}</aside>
    <button
      type="button"
      class="side-scrim"
      aria-label="メニューを閉じる"
      onclick={onclosedrawer}
    ></button>
    <main class="main">{@render children()}</main>
  </div>
</div>

<style>
  .frame {
    container: frame / inline-size;
    height: 100%;
  }
  .app {
    display: grid;
    grid-template-columns: var(--sidebar-w) minmax(0, 1fr);
    height: 100%;
  }
  .side {
    background: var(--sidebar);
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px 10px 10px max(10px, env(safe-area-inset-left, 0px));
    overflow-y: auto;
    min-height: 0;
  }
  .side-scrim {
    display: none;
    border: 0;
    padding: 0;
  }
  .main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    height: 100%;
  }
  @container frame (max-width: 860px) {
    .app {
      grid-template-columns: minmax(0, 1fr);
    }
    .side {
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      width: min(290px, 86vw);
      z-index: 50;
      transform: translateX(-102%);
      visibility: hidden;
      transition:
        transform 0.18s,
        visibility 0s 0.18s;
      padding-top: max(12px, env(safe-area-inset-top, 0px));
    }
    .drawer-open .side {
      transform: none;
      visibility: visible;
      transition: transform 0.18s;
      box-shadow: var(--shadow-dialog);
    }
    .drawer-open .side-scrim {
      display: block;
      position: fixed;
      inset: 0;
      background: var(--scrim);
      z-index: 49;
    }
  }
</style>
