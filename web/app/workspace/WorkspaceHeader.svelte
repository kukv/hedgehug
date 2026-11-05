<script lang="ts">
  import type { Snippet } from "svelte";
  import { hasPlans } from "../../core/planning";
  import type { WorkspaceEntry } from "../../core/stores";
  import Icon from "../icons/Icon.svelte";
  import { router, type WorkspaceTab } from "../router";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";

  type Props = { entry: WorkspaceEntry; tab: WorkspaceTab; actions?: Snippet };
  let { entry, tab, actions }: Props = $props();

  const at = (t: WorkspaceTab) =>
    router.href({ name: "workspace", store: entry.store, ulid: entry.ulid, tab: t });
  const items = $derived([
    { label: "概要", href: at("overview"), current: tab === "overview" },
    { label: "ツリー", href: at("tree"), current: tab === "tree" },
    { label: "カンバン", href: at("kanban"), current: tab === "kanban" },
    { label: "テーブル", href: at("table"), current: tab === "table" },
    ...(hasPlans(entry)
      ? [
          { label: "バックログ", href: at("backlog"), current: tab === "backlog" },
          { label: "スプリント", href: at("sprints"), current: tab === "sprints" },
          { label: "リリース", href: at("releases"), current: tab === "releases" },
          { label: "ガント", href: at("gantt"), current: tab === "gantt" },
          { label: "レポート", href: at("reports"), current: tab === "reports" },
        ]
      : []),
  ]);
  const end = $derived({
    label: "設定",
    href: at("settings"),
    current: tab === "settings",
    icon: "settings" as const,
  });
</script>

<header class="head">
  <div class="title">
    <StoreBadge store={entry.store} tone={entry.tone} />
    <Icon name="workspace" size={16} />
    <h1>{entry.name}</h1>
    <span class="prefix" title="ID の接頭辞">{entry.prefix}</span>
    <span class="eyebrow">ワークスペース</span>
  </div>
  {#if actions}
    <div class="actions">{@render actions()}</div>
  {/if}
</header>
<Tabs {items} {end} />

<style>
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-12);
    padding: var(--space-16) var(--gutter) 0;
  }
  .title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .prefix {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .eyebrow {
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
</style>
