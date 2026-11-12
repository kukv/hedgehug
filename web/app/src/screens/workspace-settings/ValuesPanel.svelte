<script lang="ts">
  import {
    type Category,
    deleteDialog,
    readOnlyNote,
    type SettingsState,
    statusHelp,
    valueRows,
    valuesHelp,
  } from "@hedgehug/core";
  import { Button, Input } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import CategoryDefaults from "./CategoryDefaults.svelte";
  import DeleteValueDialog from "./DeleteValueDialog.svelte";
  import ValueRow from "./ValueRow.svelte";

  // 値の編集の一覧(03「値の編集」)。list は "statuses"・"priorities"・"systems"、または選択肢のプロパティの ID。
  interface Props {
    state: SettingsState;
    list: string;
  }

  let { state: s, list }: Props = $props();

  let newName = $state("");
  let deleting = $state<string | null>(null);

  const rows = $derived(valueRows(s.detail.settings, list, s.usage));
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const model = $derived(
    deleting === null ? null : deleteDialog(s.detail.settings, list, deleting, s.usage),
  );

  const run = (op: Parameters<typeof workspaceSettings.run>[0], ok?: string) =>
    workspaceSettings.run(op, ok);

  async function add() {
    const name = newName;
    if (await run((p, st, w) => p.addValue(st, w, list, name))) newName = "";
  }
  function addKey(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void add();
  }
  // 断りはエラーのトーストなので、ダイアログを閉じてから run する。
  function remove(replacement: string) {
    const id = deleting;
    const name = rows.find((r) => r.id === id)?.name ?? "";
    deleting = null;
    if (id === null) return;
    void run(
      (p, st, w) => p.deleteValue(st, w, list, id, replacement),
      `「${name}」を削除しました`,
    );
  }
</script>

<div class="panel">
  <p class="help">{list === "statuses" ? statusHelp : valuesHelp}</p>
  {#if list === "statuses"}
    <CategoryDefaults state={s} />
  {/if}
  <div class="rows">
    {#each rows as row (row.id)}
      <ValueRow
        {row}
        {list}
        readOnly={s.readOnly}
        onrename={(name) => run((p, st, w) => p.renameValue(st, w, list, row.id, name))}
        oncolor={(color) => run((p, st, w) => p.setValueColor(st, w, list, row.id, color))}
        oncategory={(c: Category) => run((p, st, w) => p.setStatusCategory(st, w, row.id, c))}
        oninitial={() =>
          run((p, st, w) =>
            list === "statuses"
              ? p.setInitialStatus(st, w, row.id)
              : p.setInitialPriority(st, w, row.id),
          )}
        onmove={(d) => run((p, st, w) => p.moveValue(st, w, list, row.id, d))}
        ondelete={() => {
          if (workspaceSettings.askDelete(list, row.id)) deleting = row.id;
        }}
      />
    {/each}
  </div>
  <div class="add">
    <Input
      label="新しい値の名前"
      placeholder="新しい値の名前"
      bind:value={newName}
      disabled={s.readOnly}
      {title}
      onkeydown={addKey}
    />
    <Button disabled={s.readOnly} {title} onclick={add}>追加</Button>
  </div>
</div>

{#if model}
  <DeleteValueDialog {model} onconfirm={remove} onclose={() => (deleting = null)} />
{/if}

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 720px;
  }
  .help {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
  .add {
    display: flex;
    gap: 8px;
  }
</style>
