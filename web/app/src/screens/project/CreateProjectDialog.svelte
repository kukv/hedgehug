<script lang="ts">
  import { type ProjectStatus, projectStatuses } from "@hedgehug/core";
  import { Button, Dialog, Input, Select } from "@hedgehug/ui";
  import { projectDialog } from "../../wire";

  // プロジェクトの作成ダイアログ(02「プロジェクト」の「作成」)。値と送信は core の projectDialog が持つ。
  const dialog = projectDialog.state;
  const statuses = projectStatuses.map((s) => ({ value: s, label: s }));

  function onkeydown(event: KeyboardEvent) {
    // Enter で作成する(02)。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void projectDialog.submit();
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="プロジェクトを作成" onclose={projectDialog.close}>
    <div class="form">
      <div class="field">
        <span>ワークスペース</span>
        <Select
          label="ワークスペース"
          options={s.workspaces}
          bind:value={
            () => s.form.workspaceId,
            (v) => void projectDialog.update({ workspaceId: v })
          }
        />
      </div>
      <div class="field">
        <span>タイトル</span>
        <Input
          label="タイトル"
          focusOnMount
          {onkeydown}
          bind:value={() => s.form.title, (v) => void projectDialog.update({ title: v })}
        />
      </div>
      <div class="field">
        <span>オーナー</span>
        <Select
          label="オーナー"
          options={s.owners}
          bind:value={() => s.form.owner, (v) => void projectDialog.update({ owner: v })}
        />
      </div>
      <div class="field">
        <span>ステータス</span>
        <Select
          label="ステータス"
          options={statuses}
          bind:value={
            () => s.form.status,
            (v) => void projectDialog.update({ status: v as ProjectStatus })
          }
        />
      </div>
      <div class="dates">
        <div class="field">
          <span>開始日</span>
          <Input
            label="開始日"
            type="date"
            bind:value={() => s.form.start, (v) => void projectDialog.update({ start: v })}
          />
        </div>
        <div class="field">
          <span>終了日</span>
          <Input
            label="終了日"
            type="date"
            bind:value={() => s.form.end, (v) => void projectDialog.update({ end: v })}
          />
        </div>
      </div>
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={projectDialog.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void projectDialog.submit()}>作成</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
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
  .dates {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
