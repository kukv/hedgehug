<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { hasPlans } from "../../core/planning";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { onLinkClick, type ProjectTab, router } from "../router";
  import { useDisplayId } from "../svelte-store";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";

  // The heading of a project: where it is (Store, workspace), its name and ID, and the tabs.
  // The settings of the project are the tab at the right end.
  // The project is not there until it is read: the heading has no title then (as the tree's had),
  // and the tabs, which only need the ULID, are drawn at once.
  type Props = { ulid: string; project?: TaskView; workspace?: WorkspaceEntry; tab: ProjectTab };
  let { ulid, project, workspace, tab }: Props = $props();

  const c = core();
  const did = useDisplayId(c.stores);
  const at = (t: ProjectTab) => router.href({ name: "project", ulid, tab: t });
  const items = $derived([
    { label: "概要", href: at("overview"), current: tab === "overview" },
    { label: "ツリー", href: at("tree"), current: tab === "tree" },
    { label: "カンバン", href: at("kanban"), current: tab === "kanban" },
    { label: "テーブル", href: at("table"), current: tab === "table" },
    // As on the workspace: a chart for those that plan with sprints or releases.
    ...(workspace && hasPlans(workspace)
      ? [{ label: "ガント", href: at("gantt"), current: tab === "gantt" }]
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
  <nav class="crumb" aria-label="パンくず">
    {#if project}
      <StoreBadge store={project.store} tone={c.stores.tone(project.store)} />
    {/if}
    {#if workspace}
      {@const wsHref = router.href({
        name: "workspace",
        store: workspace.store,
        ulid: workspace.ulid,
        tab: "overview",
      })}
      <a class="ws" href={wsHref} onclick={(e) => onLinkClick(e, wsHref)}>
        <Icon name="workspace" size={16} />{workspace.name}
      </a>
      <span class="sep" aria-hidden="true">›</span>
    {/if}
    <KindIcon kind="project" />
    <h1 tabindex="-1">{project?.title ?? ""}</h1>
    {#if project}
      <span class="id">{did.current(project.store, project.id)}</span>
    {/if}
  </nav>
</header>
<Tabs {items} {end} />

<style>
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
    padding: 1.125rem 1.75rem 0;
  }
  .crumb {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.625rem;
    min-width: 0;
  }
  .ws {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    color: var(--ink-2);
    text-decoration: none;
  }
  .sep {
    color: var(--muted-2);
  }
  h1 {
    margin: 0;
    font-size: 1.19rem;
    font-weight: 600;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--muted);
  }
</style>
