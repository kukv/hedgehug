<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { Brand, Button } from "@hedgehug/ui";
  import { SvelteSet } from "svelte/reactivity";
  import type { Links } from "../links";
  import { itemDialog } from "../wire";
  import SidebarStore from "./SidebarStore.svelte";

  // サイドバー(04「サイドバー」)。A2 は、名前・「新規作成」・Store ごとのブロックだけを出す。
  interface Props {
    stores: StoreNode[];
    current: Resolved | null;
    links: Links;
  }

  let { stores, current, links }: Props = $props();

  // 開閉は、このタブのあいだだけ覚える(このマシンに残すのはトラック G のマシンの設定が入ってから)。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
</script>

<Brand />
<Button variant="primary" onclick={() => void itemDialog.open()}>
  新規作成 <kbd class="kbd">C</kbd>
</Button>
{#each stores as store (store.id)}
  <SidebarStore {store} {current} {links} {collapsed} ontoggle={toggle} />
{/each}

<style>
  .kbd {
    font-family: var(--font-mono);
    font-size: 11px;
    opacity: 0.75;
    margin-left: 4px;
  }
</style>
