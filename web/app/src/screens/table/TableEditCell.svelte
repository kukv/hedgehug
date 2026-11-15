<script lang="ts">
  import {
    assigneeText,
    type Category,
    type CellEdit,
    cellEditor,
    cellView,
    deletedValue,
    readOnlyNote,
    type SearchColumn,
    type SearchRow,
    type TableCells,
  } from "@hedgehug/core";
  import { Avatar, Select, StatusDot } from "@hedgehug/ui";
  import PropertyField from "../item/PropertyField.svelte";
  import TableCell from "./TableCell.svelte";

  // テーブルの編集できる列のセル 1 つ(04「セル」、E2b)。cellEditor が決めた部品(プロパティの入力欄・スプリントとリリースの
  // セレクト)を出す。選択肢の元が読めていない・削除済みの行・この行で使わない列は、E2a の表示のまま。
  interface Props {
    row: SearchRow;
    column: SearchColumn;
    now: Date;
    cells: TableCells;
    reload: () => Promise<void>;
  }

  let { row, column, now, cells, reload }: Props = $props();

  // 元を読み終えるたびに引き直す(context は読めるまで undefined)。
  let loaded = $state(0);
  $effect(() => cells.loaded.subscribe((n) => (loaded = n)));
  const ctx = $derived.by(() => {
    void loaded;
    return cells.context(row.storeId, row.workspaceId);
  });
  const editor = $derived(cellEditor(row, column, ctx));
  const num = $derived(
    column.key === "points" || column.key === "estimate" || column.type === "number",
  );
  const category = $derived(row.status?.category as Category | undefined);
  const who = $derived(assigneeText(row.assignee));
  let picked = $state("");
  // 選択肢の元の読み込みなどで editor が作り直されても、値そのものが変わるまでは選んだ表示を替えない。
  const planValue = $derived(editor.kind === "plan" ? editor.value : "");
  $effect(() => {
    picked = planValue;
  });

  const save = (edit: CellEdit) => cells.save(row, edit, reload);
  async function pickPlan(key: "sprint" | "release", id: string) {
    // 保存できなかったら、選び直す前の表示に戻す。
    if (!(await save({ kind: "plan", key, id })) && editor.kind === "plan") picked = editor.value;
  }
</script>

{#if editor.kind === "property" && ctx}
  {@const field = editor.row}
  <td class:num>
    <PropertyField
      row={field}
      project={row.projectId}
      settings={ctx.settings}
      disabled={row.readOnly}
      onchange={(value) => void save({ kind: "property", propertyId: field.property.id, value })}
      ontoggle={(valueId, on) =>
        void save({ kind: "toggle", propertyId: field.property.id, valueId, on })}
    />
  </td>
{:else if editor.kind === "plan"}
  {@const plan = editor}
  <td>
    <Select
      label={column.name}
      disabled={row.readOnly}
      title={row.readOnly ? readOnlyNote : undefined}
      options={plan.options}
      bind:value={picked}
      onchange={(id) => void pickPlan(plan.key, id)}
    />
  </td>
{:else if column.key === "status"}
  <td>
    {#if row.removed.includes("status")}
      <span class="faint">{deletedValue}</span>
    {:else if row.status}
      <span class="with-icon">
        <StatusDot category={category ?? "todo"} label={row.status.name} />{row.status.name}
      </span>
    {/if}
  </td>
{:else if column.key === "assignee"}
  <td>
    {#if row.unused.includes("assignee")}
      <span class="faint">—</span>
    {:else if who !== null}
      <span class="with-icon"><Avatar name={row.assignee?.name ?? ""} title={who} />{who}</span>
    {/if}
  </td>
{:else}
  <TableCell view={cellView(row, column, now)} {column} />
{/if}

<style>
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    white-space: nowrap;
  }
  .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .faint {
    color: var(--muted);
  }
  .with-icon {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
