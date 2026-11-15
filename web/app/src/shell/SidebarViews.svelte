<script lang="ts">
  import type { Resolved, Route } from "@hedgehug/core";
  import type { Links } from "../links";
  import { views } from "../views";

  // サイドバーの「保存したビュー」(04「サイドバー」、E3b-15)。一覧は名前順(サーバーの返す順)。
  interface Props {
    current: Resolved | null;
    links: Links;
  }

  let { current, links }: Props = $props();

  const list = views.list;
  const route = (id: string): Route => ({ name: "view", id });
  const isCurrent = (id: string) => current?.name === "view" && current.viewId === id;
</script>

<section class="views" aria-label="保存したビュー">
  <h2 class="head">保存したビュー</h2>
  {#each $list as v (v.id)}
    <a
      class="row"
      href={links.href(route(v.id))}
      aria-current={isCurrent(v.id) ? "page" : undefined}
      onclick={(e) => links.click(e, route(v.id))}
      onauxclick={(e) => links.click(e, route(v.id))}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M3 5h18M3 12h18M3 19h12" />
      </svg>
      <span class="name">{v.name}</span>
    </a>
  {:else}
    <p class="empty">検索の画面から保存できます</p>
  {/each}
</section>

<style>
  .views {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding-top: 8px;
  }
  .head {
    margin: 0;
    padding: 0 6px 4px;
    font-size: 11px;
    font-weight: 500;
    color: var(--muted);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 28px;
    padding: 3px 6px;
    border-radius: 6px;
    color: var(--ink);
    text-decoration: none;
  }
  .row:hover,
  .row[aria-current="page"] {
    background: var(--hover);
  }
  .row[aria-current="page"] {
    font-weight: 500;
  }
  .row svg {
    flex: none;
    stroke: var(--muted);
    fill: none;
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .empty {
    color: var(--muted);
    font-size: 12px;
    margin: 4px 6px;
  }
</style>
