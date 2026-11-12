<script lang="ts">
  import { readOnlyNote, type SettingsState, workflowHelp, workflowRows } from "@hedgehug/core";
  import { Checkbox, StatusDot } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";

  // ワークフローの表(03「遷移の制約」)。行が From、列が To。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  const rows = $derived(workflowRows(s.detail.settings));
  const title = $derived(s.readOnly ? readOnlyNote : undefined);

  function change(from: string, to: string, checked: boolean) {
    void workspaceSettings.run((p, st, w) => p.setTransition(st, w, from, to, checked));
  }
</script>

<div class="panel">
  <p class="help">{workflowHelp}</p>
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
                    disabled={s.readOnly}
                    {title}
                    bind:checked={() => cell.checked, (v) => change(row.from, cell.to, v)}
                  />
                {/if}
              </td>
            {/each}
            <td class="note">{row.note}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .help {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
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
