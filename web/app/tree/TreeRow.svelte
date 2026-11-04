<script lang="ts">
  import type { Attachment } from "svelte/attachments";
  import type { LiveMark } from "../../core/cache";
  import { formatDate } from "../../core/format";
  import type { SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import Avatar from "../ui/Avatar.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import type { FlatRow } from "./flatten";
  import { problemLabel, propText } from "./marks";

  type Props = {
    row: FlatRow;
    schema: SchemaView | undefined;
    selected: boolean;
    focusable: boolean; // tabindex 0: one row of the tree takes the Tab key
    live: LiveMark | undefined;
    blockedBy: string | undefined; // the ID of the first blocker, when the task is blocked
    onselect: () => void;
    ontoggle: () => void;
    onopen: () => void;
    onmore?: () => void; // the "…" button: the menu of the row
    dnd?: Attachment<HTMLElement>; // makes the row draggable and a drop target
    dragging?: boolean; // this row is being dragged
    dropMark?: "before" | "after" | "child" | "reject" | null; // what a drop on this row would do
    storeTone?: StoreTone; // set when the view lists the tasks of several stores: the row shows its store
  };
  let {
    row,
    schema,
    selected,
    focusable,
    live,
    blockedBy,
    onselect,
    ontoggle,
    onopen,
    onmore,
    dnd,
    dragging = false,
    dropMark = null,
    storeTone,
  }: Props = $props();

  const task = $derived(row.task);
  const category = $derived(schema?.statuses?.find((s) => s.name === task.status)?.category);
  const role = $derived(schema?.kinds?.find((k) => k.name === task.kind)?.role);
  const closed = $derived(category === "done" || category === "canceled");
  const problem = $derived(task.problems?.[0]);
  const due = $derived(propText(task, "due"));
  const priority = $derived(propText(task, "priority"));
  const assignee = $derived(task.assignees?.[0]);
  // The keyboard is on the row (not the pointer): the ULID shows, to read. Copying it by key is
  // left to the task detail, which always shows it.
  let keyboardOn = $state(false);
  const progress = $derived(task.progress && task.progress.total > 0 ? task.progress : undefined);
  const percent = $derived(progress ? Math.round((progress.done / progress.total) * 100) : 0);
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="row"
  class:selected
  class:dragging
  data-drop={dropMark ?? undefined}
  role="treeitem"
  aria-level={row.depth + 1}
  aria-selected={selected}
  aria-expanded={row.hasChildren ? row.expanded : undefined}
  tabindex={focusable ? 0 : -1}
  data-ulid={row.ulid}
  onclickcapture={(e) => {
    // A click anywhere in the row selects it and puts the focus on it, also a click on the ID or
    // the chevron, which stop the click and the keys: the focus left on those buttons would keep
    // the row keys from the tree.
    onselect();
    e.currentTarget.focus();
  }}
  ondblclick={onopen}
  onfocus={(e) => (keyboardOn = e.currentTarget.matches(":focus-visible"))}
  onblur={() => (keyboardOn = false)}
  {@attach dnd}
>
  <span class="lead" style:padding-left="{0.4 + row.depth * 1.6}rem">
    {#if row.hasChildren}
      <button
        type="button"
        class="chev"
        tabindex="-1"
        aria-label="{row.expanded ? "折りたたむ" : "展開する"}: {task.title}"
        aria-expanded={row.expanded}
        onclick={(e) => {
          e.stopPropagation();
          ontoggle();
        }}
        ondblclick={(e) => e.stopPropagation()}
        onkeydown={(e) => e.stopPropagation()}
      >
        <Icon name={row.expanded ? "chevron-down" : "chevron-right"} size={14} />
      </button>
    {:else}
      <span class="chev-gap"></span>
    {/if}
    <StatusIcon status={task.status} {category} />
    <KindIcon kind={task.kind} {role} />
    <IdLabel id={task.id} ulid={task.ulid} store={task.store} reveal={keyboardOn} />
    <span class="title" class:closed class:container={role === "container"}>{task.title}</span>

    {#if progress}
      <span class="progress">
        <span class="bar"><span class="fill" style:width="{percent}%"></span></span>
        {progress.done}/{progress.total}
      </span>
    {:else if row.hasChildren && !row.expanded}
      <span class="progress">子 {row.childCount}</span>
    {/if}
    {#if task.checklist && task.checklist.total > 0}
      <span class="check" title="チェックリスト">
        <Icon name="checklist" size={10} />{task.checklist.done}/{task.checklist.total}
      </span>
    {/if}
    {#if live}
      <span class="live">{live.label}</span>
    {/if}
    {#if task.broken}
      <span class="warn" title={task.broken}> <Icon name="warning" size={11} />壊れています </span>
    {:else if problem}
      <span class="warn" title={problem.message}>
        <Icon name="warning" size={11} />{problemLabel(problem)}
      </span>
    {/if}
    {#if dropMark === "reject"}
      <span class="refuse"><Icon name="forbid" size={11} />ここには置けません</span>
    {/if}
  </span>

  <span class="tail">
    {#if storeTone}
      <StoreBadge store={task.store} tone={storeTone} compact />
    {/if}
    {#if blockedBy}
      <span class="blocked"><Icon name="forbid" size={12} />{blockedBy} 待ち</span>
    {/if}
    {#if priority}
      <span class="priority {priority}">{priority}</span>
    {/if}
    <span class="due">{due ? formatDate(due, new Date()) : ""}</span>
    <span class="who">
      {#if assignee}
        <Avatar name={assignee} />
      {/if}
    </span>
    {#if onmore}
      <button
        type="button"
        class="more"
        tabindex="-1"
        aria-label="メニュー: {task.title}"
        aria-haspopup="menu"
        onclick={(e) => {
          e.stopPropagation();
          onmore();
        }}
        ondblclick={(e) => e.stopPropagation()}
        onkeydown={(e) => e.stopPropagation()}
      >
        <Icon name="more" size={14} />
      </button>
    {:else}
      <!-- The button comes with the schema: its place is kept so the columns do not move. -->
      <span class="more-slot" aria-hidden="true"></span>
    {/if}
  </span>
</div>

<style>
  .row {
    position: relative;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-height: 2.375rem;
    padding-right: 0.75rem;
    border-radius: var(--radius-4);
    outline: none;
  }
  .row:hover {
    background: var(--sunken);
  }
  .row.selected {
    background: var(--accent-weak);
  }
  .row:focus-visible {
    box-shadow: inset 0 0 0 var(--hairline) var(--accent);
  }
  .row.dragging {
    opacity: 0.45;
  }
  .row[data-drop="before"]::before,
  .row[data-drop="after"]::after {
    content: "";
    position: absolute;
    left: 0;
    right: 0;
    height: calc(var(--hairline) * 2);
    border-radius: var(--radius-4);
    background: var(--accent);
    pointer-events: none;
  }
  .row[data-drop="before"]::before {
    top: 0;
  }
  .row[data-drop="after"]::after {
    bottom: 0;
  }
  .row[data-drop="child"] {
    background: var(--accent-weak);
    box-shadow: inset 0 0 0 calc(var(--hairline) * 1.5) var(--accent);
  }
  .row[data-drop="reject"] {
    background: var(--danger-weak);
    box-shadow: inset 0 0 0 calc(var(--hairline) * 1.5) var(--danger);
  }
  .refuse {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    height: 1.25rem;
    padding: 0 0.5rem;
    border-radius: var(--radius-pill);
    background: var(--danger);
    color: var(--surface);
    font-size: 0.72rem;
    font-weight: 500;
  }
  .lead {
    flex: 1 1 auto;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .chev {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.375rem;
    height: 1.375rem;
    border: 0;
    border-radius: var(--radius-4);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }
  .chev-gap {
    flex: 0 0 auto;
    width: 1.375rem;
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink);
  }
  .title.container {
    font-weight: 600;
  }
  .title.closed {
    color: var(--muted);
    font-weight: 400;
  }
  .progress {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    font-family: var(--font-mono);
    font-size: 0.7rem;
    color: var(--ink-2);
  }
  .bar {
    display: flex;
    width: 2.75rem;
    height: 0.25rem;
    border-radius: var(--radius-4);
    background: var(--line);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: var(--success);
  }
  .check,
  .live,
  .warn,
  .blocked {
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    height: 1.125rem;
    padding: 0 0.375rem;
    font-size: 0.7rem;
  }
  .check {
    border-radius: var(--radius-4);
    background: var(--hover);
    color: var(--ink-2);
    font-family: var(--font-mono);
  }
  .live {
    border-radius: var(--radius-pill);
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .warn {
    border-radius: var(--radius-pill);
    background: var(--warning-weak);
    color: var(--warning);
  }
  .tail {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.5rem;
  }
  .blocked {
    height: 1.375rem;
    padding: 0 0.5rem;
    border-radius: var(--radius-pill);
    background: var(--danger-weak);
    color: var(--danger);
    font-size: 0.72rem;
  }
  .priority {
    display: inline-flex;
    align-items: center;
    height: 1.375rem;
    padding: 0 0.5rem;
    border-radius: var(--radius-pill);
    background: var(--hover);
    color: var(--ink-2);
    font-size: 0.72rem;
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
    width: 2.75rem;
    text-align: right;
    font-family: var(--font-mono);
    font-size: 0.72rem;
    color: var(--ink-2);
  }
  .who {
    width: 1.5rem;
    display: flex;
    justify-content: center;
  }
  .more,
  .more-slot {
    flex: 0 0 auto;
    width: 1.75rem;
  }
  .more {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 1.75rem;
    border: 0;
    border-radius: var(--radius-4);
    background: transparent;
    color: var(--muted);
    opacity: 0;
    cursor: pointer;
  }
  .row:hover .more,
  .row.selected .more,
  .more:focus-visible {
    opacity: 1;
  }
  .more:hover {
    background: var(--hover);
  }
</style>
