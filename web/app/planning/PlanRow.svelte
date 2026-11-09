<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { categoryOf } from "../../core/categories";
  import { pointsOf } from "../../core/planning";
  import type { SchemaView } from "../../core/schema";
  import { statusColorOf } from "../../core/schema-edit";
  import { core } from "../context";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import { useDisplayId } from "../svelte-store";
  import Avatar from "../ui/Avatar.svelte";
  import Menu, { type MenuItem } from "../ui/Menu.svelte";
  import { type Edge, rowDrag, rowDrop } from "./dnd";

  type Props = {
    task: TaskView;
    schema: SchemaView | undefined;
    locked: boolean;
    moves: MenuItem[];
    onmove: (value: string) => void;
    ondrop?: (dragged: string, edge: Edge) => void;
  };
  let { task, schema, locked, moves, onmove, ondrop }: Props = $props();

  const did = useDisplayId(core().stores);
  const href = $derived(router.href({ name: "task", ulid: task.ulid }));
  const points = $derived(pointsOf(task));
  const assignee = $derived(task.assignees?.[0]);
  let edge = $state<Edge | null>(null);
  let button = $state<HTMLElement | null>(null);
  let menu = $state(false);

  const drop = $derived(
    ondrop
      ? rowDrop(() => ({ ulid: task.ulid, onOver: (e) => (edge = e), onDrop: ondrop }))
      : () => undefined,
  );
</script>

<li
  class="row"
  class:top={edge === "top"}
  class:bottom={edge === "bottom"}
  data-ulid={task.ulid}
  {@attach rowDrag(() => ({ ulid: task.ulid, locked }))}
  {@attach drop}
>
  <StatusIcon
    status={task.status}
    category={schema ? categoryOf(schema, task.status) : undefined}
    color={statusColorOf(schema, task.status)}
    size={14}
  />
  <KindIcon kind={task.kind} size={13} />
  <span class="id">{did.current(task.store, task.id)}</span>
  <a {href} draggable="false" onclick={(e) => onLinkClick(e, href)}>{task.title}</a>
  {#if task.project}
    <span class="project">{task.project.title}</span>
  {/if}
  {#if assignee}
    <Avatar name={assignee} />
  {/if}
  <span class="pt" class:none={points === null}>{points === null ? "—" : points} pt</span>
  {#if !locked}
    <button
      bind:this={button}
      type="button"
      class="more"
      aria-label="{task.title} の移動先"
      aria-haspopup="menu"
      aria-expanded={menu}
      onclick={() => (menu = !menu)}
    >
      →
    </button>
    {#if menu}
      <Menu
        anchor={button}
        items={moves}
        label="移動先"
        onselect={(v) => {
          menu = false;
          onmove(v);
        }}
        onclose={() => (menu = false)}
      />
    {/if}
  {/if}
</li>

<style>
  .row {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    min-height: 40px;
    padding: 0 var(--space-8);
    border-top: 1px solid var(--line);
    background: var(--surface);
  }
  .row.top {
    box-shadow: inset 0 2px 0 var(--accent);
  }
  .row.bottom {
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  .id {
    flex: 0 0 auto;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  a {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    color: var(--ink);
    text-decoration: none;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  a:hover {
    text-decoration: underline;
  }
  .project {
    flex: 0 1 auto;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-12);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pt {
    flex: 0 0 auto;
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .pt.none {
    color: var(--muted);
  }
  .more {
    flex: 0 0 auto;
    width: 26px;
    height: 26px;
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    cursor: pointer;
  }
  .more:hover {
    background: var(--sunken);
  }
</style>
