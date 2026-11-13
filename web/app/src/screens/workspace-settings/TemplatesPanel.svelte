<script lang="ts">
  import {
    deleteTemplateBody,
    deleteTemplateTitle,
    type RuleProject,
    readOnlyNote,
    type SettingsState,
    templateDeleted,
    templateDeleteLines,
    templateRows,
    templatesEmpty,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, IconButton, KindIcon, type KindName } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import KindDefaults from "./KindDefaults.svelte";
  import TemplateDialog from "./TemplateDialog.svelte";

  // テンプレートのタブ(03「テンプレート」)。
  interface Props {
    state: SettingsState;
    projects: RuleProject[];
  }

  let { state: s, projects }: Props = $props();

  // 編集ダイアログを開いているテンプレート。ID は編集、"new" は新規。
  let editing = $state<string | "new" | null>(null);
  let deleting = $state<string | null>(null);

  const rows = $derived(templateRows(s.detail.settings));
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const editingTemplate = $derived(
    s.detail.settings.templates.find((t) => t.id === editing) ?? null,
  );
  const deletingName = $derived(rows.find((r) => r.id === deleting)?.name ?? "");

  // 断りはエラーのトーストなので、ダイアログを閉じてから run する。
  function remove() {
    const id = deleting;
    deleting = null;
    if (id === null) return;
    void workspaceSettings.run((p, st, w) => p.deleteTemplate(st, w, id), templateDeleted);
  }
</script>

<div class="panel">
  <KindDefaults state={s} />
  <div class="rows">
    {#each rows as row (row.id)}
      <div class="row">
        <KindIcon kind={row.kind as KindName} />
        <span class="name">{row.name}</span>
        {#if row.summary}
          <span class="summary">{row.summary}</span>
        {/if}
        <span class="grow"></span>
        <Button onclick={() => (editing = row.id)}>編集</Button>
        <IconButton
          label="削除"
          size="sm"
          disabled={s.readOnly}
          {title}
          onclick={() => (deleting = row.id)}
          >🗑</IconButton
        >
      </div>
    {:else}
      <p class="empty">{templatesEmpty}</p>
    {/each}
  </div>
  <div>
    <Button disabled={s.readOnly} {title} onclick={() => (editing = "new")}
      >テンプレートを作成</Button
    >
  </div>
</div>

{#if editing !== null}
  <TemplateDialog state={s} template={editingTemplate} onclose={() => (editing = null)} />
{/if}

{#if deleting !== null}
  <ConfirmDialog
    title={deleteTemplateTitle}
    lines={[
      deleteTemplateBody(deletingName),
      ...templateDeleteLines(s.detail.settings, deleting, projects),
    ]}
    ok="削除"
    danger
    onok={remove}
    oncancel={() => (deleting = null)}
  />
{/if}

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 720px;
  }
  .rows {
    display: flex;
    flex-direction: column;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  .name {
    font-weight: 600;
  }
  .summary,
  .empty {
    font-size: 12.5px;
    color: var(--muted);
  }
  .empty {
    margin: 0;
  }
  .grow {
    flex: 1;
  }
</style>
