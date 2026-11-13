<script lang="ts">
  import { type ProjectWorkflowRow, readOnlyNote, type WorkflowRow } from "@hedgehug/core";
  import { Checkbox, StatusDot } from "@hedgehug/ui";

  // ワークフローの表。行が From、列が To。ワークスペースの表(WorkflowRow)とプロジェクトの表(ProjectWorkflowRow)で共通。
  interface Props {
    rows: (WorkflowRow | ProjectWorkflowRow)[];
    readOnly: boolean;
    onchange: (from: string, to: string, checked: boolean) => void;
  }

  let { rows, readOnly, onchange }: Props = $props();
</script>

<div class="scroll">
  <table>
    <thead>
      <tr>
        <th></th>
        {#each rows as row (row.from)}
          <th>{row.name}</th>
        {/each}
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each rows as row (row.from)}
        <tr>
          <th class="from"><StatusDot category={row.category} label={row.name} />{row.name}</th>
          {#each row.cells as cell (cell.to)}
            <td class="cell">
              {#if cell.self}
                —
              {:else}
                <Checkbox
                  label={`${row.name} から ${cell.name}`}
                  disabled={readOnly || ("disabled" in cell && cell.disabled)}
                  title={readOnly ? readOnlyNote : "reason" in cell ? cell.reason : undefined}
                  bind:checked={() => cell.checked, (v) => onchange(row.from, cell.to, v)}
                />
              {/if}
            </td>
          {/each}
          <td class="note">{row.note}{"blocked" in row && row.blocked ? ` ${row.blocked}` : ""}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .scroll {
    overflow-x: auto;
  }
  table {
    border-collapse: collapse;
    font-size: 12.5px;
  }
  th,
  td {
    padding: 6px 12px;
    white-space: nowrap;
    text-align: center;
  }
  th {
    font-weight: 600;
    color: var(--ink-2);
  }
  .from {
    display: flex;
    align-items: center;
    gap: 8px;
    text-align: left;
  }
  .cell {
    color: var(--muted);
  }
  /* チェックボックスの文字は読み上げ用なので、画面では隠す。 */
  .cell :global(span) {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .note {
    text-align: left;
    color: var(--muted);
  }
</style>
