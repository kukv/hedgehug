<script lang="ts">
  import { type ActivityRow, type ActivityTarget, clockOf } from "@hedgehug/core";
  import { ActorIcon, StoreBadge } from "@hedgehug/ui";
  import { activityLinkOf } from "../../activity";
  import { links } from "../../wire";

  // アクティビティの 1 行(06「表示」)。時刻・実行者のアイコン・名前と本文・Store のバッジとワークスペース名と
  // アイテムの ID とプロジェクト・SHA の順に出す。名前・本文・対象の形はサーバーが作る(C4d-12)。
  interface Props {
    row: ActivityRow;
  }

  let { row }: Props = $props();

  // 実行者のない行は名前を出さず、アイコン「hh」の説明を「hedgehug」にする。
  const title = $derived(row.actor === "none" ? "hedgehug" : row.who);
</script>

{#snippet target(t: ActivityTarget, kind: "item" | "project")}
  {@const link = activityLinkOf(row, t, kind)}
  {#if link}
    <a
      class="target"
      href={links.href(link.route)}
      onclick={(e) => links.click(e, link.route, link.action)}
      onauxclick={(e) => links.click(e, link.route)}
    >
      <code class="id">{t.display}</code>
      {#if t.title}
        <span>{t.title}</span>
      {/if}
    </a>
  {:else}
    <span class="target gone">
      <code class="id">{t.display}</code>
      {#if t.title}
        <span>{t.title}</span>
      {/if}
    </span>
  {/if}
{/snippet}

<li class="row">
  <time class="clock" datetime={row.at}>{clockOf(row.at)}</time>
  <ActorIcon kind={row.actor} name={row.avatar} {title} />
  <div class="main">
    <p class="text">
      {#if row.who}
        <span class="who">{row.who}</span>
      {/if}
      <span>{row.text}</span>
    </p>
    <div class="meta">
      <StoreBadge name={row.store.name} color={row.store.color} />
      {#if row.workspace}
        <span class="workspace">{row.workspace}</span>
      {/if}
      {#if row.item}
        {@render target(row.item, "item")}
      {/if}
      {#if row.project}
        {@render target(row.project, "project")}
      {/if}
    </div>
  </div>
  <code class="sha">{row.sha}</code>
</li>

<style>
  .row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .clock {
    flex: none;
    width: 40px;
    color: var(--muted);
    font-size: 12px;
    font-variant-numeric: tabular-nums;
    line-height: 18px;
  }
  .main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .text {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .who {
    font-weight: 500;
    margin-right: 6px;
  }
  .meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    color: var(--muted);
    font-size: 12px;
  }
  .target {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
  }
  a.target:hover {
    text-decoration: underline;
  }
  .gone {
    color: var(--muted);
  }
  .id,
  .sha {
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .sha {
    flex: none;
    width: 7ch;
    color: var(--muted);
    line-height: 18px;
  }
</style>
