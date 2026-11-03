<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import { formatDate } from "../../core/format";
  import type { PatchField } from "../../core/save";
  import { canParent, parentRule, propEditorOf, type SchemaView } from "../../core/schema";
  import type { StoreTone } from "../../core/stores";
  import FieldInput from "../detail/FieldInput.svelte";
  import type { Option } from "../detail/field-types";
  import PropField from "../detail/PropField.svelte";
  import { type PropRow, statusOptions } from "../detail/prop-rows";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import IdLabel from "../ui/IdLabel.svelte";
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
    taskOf,
    onedit,
    oncommit,
  }: {
    task: TaskView;
    col: ColumnSpec;
    schema: SchemaView | undefined;
    tone: StoreTone;
    pending: boolean;
    taskOf: (ulid: string) => TaskView | undefined;
    onedit: (field: PatchField, value: unknown) => void;
    oncommit: () => void;
  } = $props();

  const field = $derived(patchFieldOf(col));
  const edit = (v: unknown) => {
    if (field) onedit(field, v);
  };
  const category = $derived(schema?.statuses?.find((s) => s.name === task.status)?.category);
  // null: no parent; undefined: there is one whose kind is not known, so no kind is ruled out.
  const parentKind = $derived(task.parent ? taskOf(task.parent)?.kind : null);
  const kindOptions = $derived<Option[]>(
    (schema?.kinds ?? []).map((k) => {
      const usable =
        k.name === task.kind ||
        !schema ||
        parentKind === undefined ||
        canParent(schema, k.name, parentKind);
      return {
        value: k.name,
        label: k.label || k.name,
        disabled: !usable,
        title: usable || !schema ? undefined : parentRule(schema, k.name),
      };
    }),
  );
  const parent = $derived(task.parent ? taskOf(task.parent) : undefined);
  const taskHref = (ulid: string) => router.href({ name: "task", ulid });

  // A property of the schema: its definition in the store of the task, or none (the store does not
  // define it: 未定義).
  const def = $derived(isBuiltinColumn(col.id) ? undefined : schema?.props[col.id]);
  const value = $derived((task.props as Record<string, unknown>)[col.id] ?? null);
  const row = $derived<PropRow | null>(
    schema && def
      ? {
          name: col.id,
          label: `${def.label || col.id}: ${task.id}`,
          editor: propEditorOf(schema, col.id),
          multi: def.multi === true,
          options: def.values ?? [],
          value,
        }
      : null,
  );
  const refs = $derived(
    (Array.isArray(value) ? value : value ? [value] : []).map(
      (v) => taskOf(String(v))?.id ?? String(v),
    ),
  );
</script>

{#if col.id === "@store"}
  <StoreBadge store={task.store} {tone} compact />
{:else if col.id === "id"}
  <IdLabel
    id={task.id}
    ulid={task.ulid}
    href={taskHref(task.ulid)}
    onopen={(e) => onLinkClick(e, taskHref(task.ulid))}
  />
{:else if col.id === "title"}
  <FieldInput
    editor="text"
    label="タイトル: {task.id}"
    value={task.title}
    nullable={false}
    {pending}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "kind"}
  <FieldInput
    editor="select"
    label="種類: {task.id}"
    value={task.kind}
    options={kindOptions}
    nullable={false}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "status"}
  <span class="status">
    <StatusIcon status={task.status} {category} />
    <FieldInput
      editor="select"
      label="ステータス: {task.id}"
      value={task.status}
      options={statusOptions(schema)}
      nullable={false}
      onedit={edit}
      {oncommit}
    />
  </span>
{:else if col.id === "assignee"}
  <FieldInput
    editor="chips"
    label="担当: {task.id}"
    value={task.assignees ?? []}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "label"}
  <FieldInput
    editor="chips"
    label="ラベル: {task.id}"
    value={task.labels ?? []}
    onedit={edit}
    {oncommit}
  />
{:else if col.id === "parent"}
  {#if parent}
    <IdLabel
      id={parent.id}
      ulid={parent.ulid}
      href={taskHref(parent.ulid)}
      onopen={(e) => onLinkClick(e, taskHref(parent.ulid))}
    />
  {:else}
    <span class="plain">—</span>
  {/if}
{:else if col.id === "created" || col.id === "updated"}
  <span class="plain mono"
    >{formatDate(col.id === "created" ? task.created : task.updated, new Date())}</span
  >
{:else if !schema}
  <span class="plain"></span>
{:else if !row}
  <span class="undef" title="{task.store} の schema.yaml にはないプロパティです">（未定義）</span>
{:else if row.editor === "taskref"}
  <span class="plain mono">{refs.join(", ")}</span>
{:else}
  <PropField {row} value={row.value} {pending} onedit={edit} {oncommit} />
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
