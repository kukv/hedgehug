<script lang="ts">
  import type { DetailView, Route } from "@hedgehug/core";
  import { IconButton, StoreBadge } from "@hedgehug/ui";
  import { detail, links } from "../../wire";

  // 詳細パネルのヘッダー: パンくず(Store・ワークスペース・プロジェクト・祖先)、全画面、閉じる(02「上から順の構成」)。
  // ウォッチとメニューは A4(A2 の計画の決定 18)。
  interface Props {
    v: DetailView;
    full: boolean;
  }

  let { v, full }: Props = $props();

  const wsRoute = $derived<Route>({
    name: "workspace",
    store: v.store.name,
    prefix: v.workspace.prefix,
    tab: "overview",
  });
  const projectRoute = $derived<Route>({
    name: "project",
    store: v.store.name,
    project: v.project.displayId,
    tab: "overview",
  });
  const itemRoute = (displayId: string): Route => ({
    name: "item",
    store: v.store.name,
    item: displayId,
  });
  const ancestorId = (n: number) => `${v.workspace.prefix}-${n}`;
</script>

<nav class="crumbs" aria-label="パンくず">
  <StoreBadge name={v.store.name} color={v.store.color} />
  <a
    href={links.href(wsRoute)}
    onclick={(e) => links.click(e, wsRoute)}
    onauxclick={(e) => links.click(e, wsRoute)}
    >{v.workspace.name}</a
  >
  <span class="sep">/</span>
  <a
    href={links.href(projectRoute)}
    onclick={(e) => links.click(e, projectRoute)}
    onauxclick={(e) => links.click(e, projectRoute)}
    >{v.project.title}</a
  >
  {#each v.ancestors as a (a.id)}
    {@const r = itemRoute(ancestorId(a.number))}
    <span class="sep">/</span>
    <a
      href={links.href(r)}
      onclick={(e) => links.click(e, r, () => void detail.open(v.store.id, a.id))}
      onauxclick={(e) => links.click(e, r)}
      >{ancestorId(a.number)}</a
    >
  {/each}
</nav>
<span class="grow"></span>
<IconButton label={full ? "パネルで表示" : "全画面で表示"} size="sm" onclick={detail.toggleFull}
  >{full ? "⤡" : "⤢"}</IconButton
>
<IconButton label="閉じる" size="sm" onclick={detail.close}>×</IconButton>

<style>
  .crumbs {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    flex-wrap: wrap;
    font-size: 12.5px;
  }
  .crumbs a {
    color: var(--muted);
    text-decoration: none;
  }
  .crumbs a:hover {
    color: var(--ink);
    text-decoration: underline;
  }
  .sep {
    color: var(--muted-2);
  }
  .grow {
    flex: 1;
  }
</style>
