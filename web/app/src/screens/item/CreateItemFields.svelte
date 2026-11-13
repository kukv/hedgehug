<script lang="ts">
  import type { ItemDialogState, Kind } from "@hedgehug/core";
  import { kinds } from "@hedgehug/core";
  import { Input, Select, Textarea } from "@hedgehug/ui";
  import { itemDialog } from "../../wire";
  import CreatePropertyFields from "./CreatePropertyFields.svelte";
  import CreateStructure from "./CreateStructure.svelte";

  // 作成ダイアログの項目(02「ダイアログの項目」のうち、種類・ワークスペース・プロジェクト・親・テンプレート・タイトル・説明・一緒に作るもの)。
  interface Props {
    s: ItemDialogState;
  }

  let { s }: Props = $props();

  const kindOptions = kinds.map((k) => ({ value: k, label: k }));

  function onkeydown(event: KeyboardEvent) {
    // Enter で作成する(02)。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void itemDialog.submit();
  }
</script>

<div class="grid">
  <div class="field">
    <span>種類</span>
    <Select
      label="種類"
      options={kindOptions}
      bind:value={() => s.form.kind, (v) => void itemDialog.update({ kind: v as Kind })}
    />
  </div>
  <div class="field">
    <span>ワークスペース</span>
    <Select
      label="ワークスペース"
      options={s.workspaces}
      bind:value={() => s.form.workspaceId, (v) => void itemDialog.update({ workspaceId: v })}
    />
  </div>
  <div class="field">
    <span>プロジェクト</span>
    <Select
      label="プロジェクト"
      options={s.projects}
      bind:value={() => s.form.projectId, (v) => void itemDialog.update({ projectId: v })}
    />
  </div>
  {#if s.form.kind !== "Epic"}
    <div class="field">
      <span>親</span>
      {#if s.parentMissing}
        <p class="form-error">{s.parentMissing}</p>
      {:else}
        <Select
          label="親"
          options={s.parents}
          bind:value={() => s.form.parentId ?? "", (v) => void itemDialog.update({ parentId: v })}
        />
      {/if}
    </div>
  {/if}
  {#if s.templates.length > 0}
    <div class="field">
      <span>テンプレート</span>
      <Select
        label="テンプレート"
        options={s.templates}
        bind:value={() => s.form.templateId, (v) => void itemDialog.update({ templateId: v })}
      />
    </div>
  {/if}
</div>
<div class="field">
  <span>タイトル</span>
  <Input
    label="タイトル"
    placeholder="{s.form.kind} のタイトル"
    focusOnMount
    {onkeydown}
    bind:value={() => s.form.title, (v) => void itemDialog.update({ title: v })}
  />
</div>
<CreatePropertyFields {s} />
<div class="field">
  <span>説明</span>
  <Textarea
    label="説明"
    rows={4}
    bind:value={() => s.form.description, (v) => void itemDialog.update({ description: v })}
  />
</div>
{#if s.structure}
  <CreateStructure structure={s.structure} />
{/if}

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
</style>
