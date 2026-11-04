<script lang="ts">
  import type { Attachment } from "svelte/attachments";
  import type { TaskView } from "../../core/api/client";
  import { formatDate } from "../../core/format";
  import type { SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { problemLabel, propText } from "../tree/marks";
  import Avatar from "../ui/Avatar.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";

  type Props = {
    task: TaskView;
    schema: SchemaView | undefined;
    blockedBy: string | undefined; // the ID of the first blocker, when the task is blocked
    tone: StoreTone | undefined; // the colour of the store, when the badge shows
    dragging: boolean;
    onopen: () => void;
    dnd: Attachment<HTMLElement>; // makes the card draggable
  };
  let { task, schema, blockedBy, tone, dragging, onopen, dnd }: Props = $props();

  const category = $derived(schema?.statuses?.find((s) => s.name === task.status)?.category);
  const role = $derived(schema?.kinds?.find((k) => k.name === task.kind)?.role);
  const closed = $derived(category === "done" || category === "canceled");
  const problem = $derived(task.problems?.[0]);
  const due = $derived(propText(task, "due"));
  const priority = $derived(propText(task, "priority"));
  const points = $derived(
    typeof task.props?.points === "number" ? (task.props.points as number) : undefined,
  );
  const kids = $derived(task.progress?.total ?? 0);
  const people = $derived((task.assignees ?? []).slice(0, 3));
  const hasMeta = $derived(
    !!(task.broken || problem || blockedBy || priority || due || kids > 0 || people.length > 0) ||
      (task.checklist?.total ?? 0) > 0,
  );
  const more = $derived(Math.max(0, (task.assignees ?? []).length - 3));
</script>

<!-- One item of the list of its cell. The title is a button that covers the whole card, so a click
  anywhere on it and Enter on the title open the task; the ID stays above it for its own buttons. -->
<li class="card" class:dragging data-ulid={task.ulid} {@attach dnd}>
  <div class="top">
    <KindIcon kind={task.kind} {role} />
    <span class="id"><IdLabel id={task.id} ulid={task.ulid} store={task.store} /></span>
    {#if tone}
      <StoreBadge store={task.store} {tone} compact />
    {/if}
    {#if points !== undefined}
      <span class="points">{points} pt</span>
    {/if}
  </div>
  <button type="button" class="title" class:closed onclick={onopen}>{task.title}</button>
  {#if hasMeta}
    <div class="meta">
      {#if task.broken}
        <span class="warn" title={task.broken}><Icon name="warning" size={11} />壊れています</span>
      {:else if problem}
        <span class="warn" title={problem.message}>
          <Icon name="warning" size={11} />{problemLabel(problem)}
        </span>
      {/if}
      {#if blockedBy}
        <span class="blocked"><Icon name="forbid" size={12} />{blockedBy} 待ち</span>
      {/if}
      {#if priority}
        <span class="priority {priority}">{priority}</span>
      {/if}
      {#if task.checklist && task.checklist.total > 0}
        <span class="chip" title="チェックリスト">
          <Icon name="checklist" size={10} />{task.checklist.done}/{task.checklist.total}
        </span>
      {/if}
      {#if kids > 0}
        <span class="chip">子 {kids}</span>
      {/if}
      {#if due}
        <span class="due">{formatDate(due, new Date())}</span>
      {/if}
      <span class="who">
        {#each people as a (a)}
          <Avatar name={a} />
        {/each}
        {#if more > 0}
          <span class="more">+{more}</span>
        {/if}
      </span>
    </div>
  {/if}
</li>

<style>
  .card {
    position: relative;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    list-style: none;
    padding: var(--space-8) var(--space-10);
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: 0 var(--hairline) 0 rgba(24, 24, 27, 0.04);
    cursor: pointer;
  }
  .card:hover {
    border-color: var(--line-input);
  }
  .card:focus-within {
    border-color: var(--accent);
  }
  .id {
    position: relative;
    z-index: 1;
  }
  .card.dragging {
    opacity: 0.4;
    border-style: dashed;
  }
  .top {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
  }
  .points {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    color: var(--ink-2);
  }
  .title {
    padding: 0;
    border: 0;
    background: transparent;
    font: inherit;
    font-size: var(--fs-13);
    line-height: 1.45;
    color: var(--ink);
    text-align: left;
    overflow-wrap: anywhere;
    cursor: pointer;
  }
  /* The button covers the card: a click anywhere on it opens the task. */
  .title::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: var(--radius-8);
  }
  .title:focus-visible {
    outline: none;
  }
  .title:focus-visible::after {
    box-shadow: inset 0 0 0 calc(var(--hairline) * 2) var(--accent);
  }
  .title.closed {
    color: var(--muted);
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-6);
  }
  .warn,
  .blocked,
  .chip,
  .priority {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    height: 18px;
    padding: 0 var(--space-6);
    font-size: var(--fs-11);
  }
  .warn {
    border-radius: var(--radius-pill);
    background: var(--warning-weak);
    color: var(--warning);
  }
  .blocked {
    border-radius: var(--radius-pill);
    background: var(--danger-weak);
    color: var(--danger);
  }
  .chip {
    border-radius: var(--radius-4);
    background: var(--hover);
    color: var(--ink-2);
    font-family: var(--font-mono);
  }
  .priority {
    border-radius: var(--radius-pill);
    background: var(--hover);
    color: var(--ink-2);
    font-weight: 500;
  }
  .priority.high {
    background: var(--warning-weak);
    color: var(--warning);
  }
  .priority.urgent {
    background: var(--danger-weak);
    color: var(--danger);
  }
  .due {
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    color: var(--ink-2);
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    margin-left: auto;
  }
  .more {
    font-size: var(--fs-11);
    color: var(--muted);
  }
</style>
