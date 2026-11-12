<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { Brand } from "@hedgehug/ui";
  import { SvelteSet } from "svelte/reactivity";
  import type { Links } from "../links";
  import SidebarStore from "./SidebarStore.svelte";

  // サイドバー(04「サイドバー」)。A2 は、名前と Store ごとのブロックだけを出す(「新規作成」は Task 9)。
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
{#each stores as store (store.id)}
  <SidebarStore {store} {current} {links} {collapsed} ontoggle={toggle} />
{/each}
