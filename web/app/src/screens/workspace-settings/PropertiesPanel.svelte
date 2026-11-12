<script lang="ts">
  import {
    deletePropertyBody,
    propertiesHelp,
    propertyRows,
    readOnlyNote,
    type SettingsState,
    toggleKind,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import AddPropertyDialog from "./AddPropertyDialog.svelte";
  import ChoicesDialog from "./ChoicesDialog.svelte";
  import DeletePropertyDialog from "./DeletePropertyDialog.svelte";
  import PropertyRow from "./PropertyRow.svelte";

  // 「プロパティ」の表(03「プロパティ」)。ダイアログを開く状態だけを持ち、操作は core の run に渡す。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  let adding = $state(false);
  let choices = $state<string | null>(null);
  let deleting = $state<string | null>(null);

  const rows = $derived(propertyRows(s.detail.settings));
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const deleteBody = $derived.by(() => {
    const row = rows.find((r) => r.id === deleting);
    return row ? deletePropertyBody(row.name, s.usage.properties[row.id] ?? 0) : "";
  });

  const run = (op: Parameters<typeof workspaceSettings.run>[0], ok?: string) =>
    workspaceSettings.run(op, ok);

  // 断りはエラーのトーストなので、ダイアログを閉じてから run する。
  function remove() {
    const id = deleting;
    deleting = null;
    if (id === null) return;
    void run((p, st, w) => p.deleteProperty(st, w, id), "プロパティを削除しました");
  }
</script>

<div class="panel">
  <p class="help">{propertiesHelp}</p>
  <div class="head">
    <span>名前</span><span>キー</span><span>型</span><span>使う種類</span><span>操作</span>
  </div>
  {#each rows as row (row.id)}
    <PropertyRow
      {row}
      readOnly={s.readOnly}
      onrename={(name) => run((p, st, w) => p.renameProperty(st, w, row.id, name))}
      onkind={(kind) =>
        run((p, st, w) => p.setPropertyKinds(st, w, row.id, toggleKind(row.kinds, kind)))}
      onhide={() => run((p, st, w) => p.setPropertyHidden(st, w, row.id, !row.hidden))}
      onmove={(d) => run((p, st, w) => p.moveProperty(st, w, row.id, d))}
      ondelete={() => (deleting = row.id)}
      onchoices={() => (choices = row.id)}
    />
  {/each}
  <div>
    <Button disabled={s.readOnly} {title} onclick={() => (adding = true)}>プロパティを追加</Button>
  </div>
</div>

{#if adding}
  <AddPropertyDialog
    onclose={() => (adding = false)}
    onadded={(id, type) => {
      adding = false;
      if (type === "single" || type === "multi") choices = id;
    }}
  />
{/if}
{#if choices !== null}
  <ChoicesDialog state={s} propertyId={choices} onclose={() => (choices = null)} />
{/if}
{#if deleting !== null}
  <DeletePropertyDialog body={deleteBody} onconfirm={remove} onclose={() => (deleting = null)} />
{/if}

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 960px;
  }
  .help {
    margin: 0 0 8px;
    font-size: 12.5px;
    color: var(--muted);
  }
  .head {
    display: grid;
    grid-template-columns: minmax(140px, 1.4fr) minmax(120px, 1fr) minmax(140px, 1fr) minmax(
        80px,
        0.8fr
      ) auto;
    gap: 12px;
    font-size: 11.5px;
    color: var(--muted);
    border-bottom: 1px solid var(--line);
    padding-bottom: 4px;
  }
</style>
