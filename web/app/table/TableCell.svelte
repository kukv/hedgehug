<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { formatDate } from "../../core/format";
  import { kindChoices } from "../../core/kinds";
  import type { PatchField } from "../../core/save";
  import { propEditorOf, type SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
  import { core } from "../context";
  import FieldInput from "../detail/FieldInput.svelte";
  import type { Option } from "../detail/field-types";
  import PropField from "../detail/PropField.svelte";
  import { type PropRow, statusOptions } from "../detail/prop-rows";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import { useDisplayId } from "../svelte-store";
  import IdLabel from "../ui/IdLabel.svelte";
  import PrChip from "../ui/PrChip.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { type ColumnSpec, isBuiltinColumn, patchFieldOf } from "./columns";

  // One cell of the table. The schema is the one of the task's store (undefined until it is read).
  // pending: the field has an edit the server has not answered.
  let {
    task,
    col,
    schema,
    tone,
    pending,
    readonly = false,
    describedby,
    taskOf,
    parentLoading = false,
    onedit,
    oncommit,
  }: {
    task: TaskView;
    col: ColumnSpec;
    schema: SchemaView | undefined;
    tone: StoreTone;
    pending: boolean;
    // The Store of the task cannot be written: the cells only read.
    readonly?: boolean;
    // The id of the text that says why the cells only read.
    describedby?: string;
    taskOf: (ulid: string) => TaskView | undefined;
    // The parent of the task is not in the cache yet (it is being read), as against not found.
    parentLoading?: boolean;
    onedit: (field: PatchField, value: unknown) => void;
    oncommit: () => void;
  } = $props();

  const field = $derived(patchFieldOf(col));
  const edit = (v: unknown) => {
    if (field) onedit(field, v);
  };
  const statusDef = $derived(schema?.statuses?.find((s) => s.name === task.status));
  const category = $derived(statusDef?.category);
  // null: no parent; undefined: there is one whose kind is not known, so no kind is ruled out.
  const parentKind = $derived(task.parent ? taskOf(task.parent)?.kind : null);
  const kindOptions = $derived<Option[]>(kindChoices(task.kind, parentKind));
  const parent = $derived(task.parent ? taskOf(task.parent) : undefined);
  const did = useDisplayId(core().stores);
  const taskId = $derived(did.current(task.store, task.id));
  const taskHref = (ulid: string) => router.href({ name: "task", ulid });

  // A property of the schema: its definition in the store of the task, or none (the store does not
  // define it: 未定義).
  const def = $derived(isBuiltinColumn(col.id) ? undefined : schema?.props[col.id]);
  const value = $derived((task.props as Record<string, unknown>)[col.id] ?? null);
  const row = $derived<PropRow | null>(
    schema && def
      ? {
          name: col.id,
          label: `${def.label || col.id}: ${taskId}`,
          editor: propEditorOf(schema, col.id),
          multi: def.multi === true,
          options: def.values ?? [],
          value,
        }
      : null,
  );
</script>

{#if col.id === "@store"}
  <StoreBadge store={task.store} {tone} compact />
{:else if col.id === "pr"}
  <PrChip links={task.links} />
{:else if col.id === "id"}
  <IdLabel
    id={task.id}
    ulid={task.ulid}
    store={task.store}
    href={taskHref(task.ulid)}
    onopen={(e) => onLinkClick(e, taskHref(task.ulid))}
  />
{:else if col.id === "title"}
  <FieldInput
    editor="text"
    label="タイトル: {taskId}"
    value={task.title}
    nullable={false}
    {pending}
    {readonly}
    {describedby}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "kind"}
  <FieldInput
    editor="select"
    label="種類: {taskId}"
    value={task.kind}
    options={kindOptions}
    nullable={false}
    {readonly}
    {describedby}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "status"}
  <span class="status">
    <StatusIcon status={task.status} {category} color={statusDef?.color} />
    <FieldInput
      editor="select"
      label="ステータス: {taskId}"
      value={task.status}
      options={statusOptions(schema)}
      nullable={false}
      {readonly}
      {describedby}
      onedit={edit}
      {oncommit}
    />
  </span>
{:else if col.id === "assignee"}
  <FieldInput
    editor="chips"
    label="担当者: {taskId}"
    value={task.assignees ?? []}
    {readonly}
    {describedby}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "label"}
  <FieldInput
    editor="chips"
    label="ラベル: {taskId}"
    value={task.labels ?? []}
    {readonly}
    {describedby}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "parent"}
  {#if parent}
    <IdLabel
      id={parent.id}
      ulid={parent.ulid}
      store={parent.store}
      href={taskHref(parent.ulid)}
      onopen={(e) => onLinkClick(e, taskHref(parent.ulid))}
    />
  {:else if !task.parent}
    <span class="plain">—</span>
  {:else if parentLoading}
    <span class="plain" role="img" aria-label="親を読み込み中">…</span>
  {:else}
    <span class="plain" role="img" aria-label="親を読み込めません" title={task.parent}>?</span>
  {/if}
{:else if col.id === "created" || col.id === "updated"}
  <span class="plain mono"
    >{formatDate(col.id === "created" ? task.created : task.updated, new Date())}</span
  >
{:else if !schema}
  <span class="plain"></span>
{:else if !row}
  <span class="undef" title="{task.store} の schema.yaml にはないプロパティです">（未定義）</span>
{:else}
  <PropField {row} value={row.value} {pending} {readonly} {describedby} onedit={edit} {oncommit} />
{/if}

<style>
  .status {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-width: 0;
  }
  .plain,
  .undef {
    display: block;
    overflow: hidden;
    padding: 0 var(--space-8);
    color: var(--ink-2);
    font-size: var(--fs-13);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .undef {
    font-style: italic;
  }
</style>
