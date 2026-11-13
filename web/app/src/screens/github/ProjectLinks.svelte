<script lang="ts">
  import { linkText, linkUrl, noLinksText, type ProjectView } from "@hedgehug/core";
  import { onDestroy } from "svelte";
  import { projectLinks } from "../../github";
  import { detail, links } from "../../wire";
  import LinkChips from "./LinkChips.svelte";

  // 「リポジトリと PR」の区画の下段「PR と Issue」(02「概要」)。行の ID を押すと、そのアイテムの詳細パネルを開く。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();
  const model = projectLinks.state;
  const storeId = $derived(view.store.id);
  const projectId = $derived(view.project.id);

  $effect(() => {
    void projectLinks.load(storeId, projectId);
  });
  onDestroy(() => projectLinks.reset());

  const route = (display: string) => ({
    name: "item" as const,
    store: view.store.name,
    item: display,
  });
</script>

<h3>PR と Issue</h3>
{#if $model.loaded && $model.rows.length === 0}
  <p class="empty">{noLinksText}</p>
{/if}
<ul class="links" aria-label="PR と Issue">
  {#each $model.rows as r (`${r.item}:${r.kind}:${r.repo}#${r.number}`)}
    <li class="link">
      <LinkChips row={r} />
      <a class="row-title" href={linkUrl(r)} target="_blank" rel="noopener noreferrer"
        >{linkText(r)}</a
      >
      <a
        class="id"
        href={links.href(route(r.display))}
        onclick={(e) => links.click(e, route(r.display), () => void detail.open(storeId, r.item))}
        onauxclick={(e) => links.click(e, route(r.display))}
        >{r.display}</a
      >
    </li>
  {/each}
</ul>

<style>
  h3 {
    font-size: 13px;
    margin: 8px 0 0;
  }
  .empty {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .links {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .link {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    overflow-wrap: anywhere;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
</style>
