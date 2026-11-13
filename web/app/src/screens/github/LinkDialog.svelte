<script lang="ts">
  import { askingText, linkDialogTitle } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input, Select } from "@hedgehug/ui";
  import { linkDialog } from "../../github";

  // 「PR をリンク」「Issue をリンク」(07「追加」)。リポジトリは、プロジェクトにあれば選び、なければ入力する。
  const dialog = linkDialog.state;

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void linkDialog.submit();
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title={linkDialogTitle(s.kind)} onclose={linkDialog.close}>
    <div class="form">
      <div class="field">
        <span>リポジトリ</span>
        {#if s.repos.length > 0}
          <Select
            label="リポジトリ"
            options={s.repos.map((r) => ({ value: r, label: r }))}
            bind:value={() => s.repo, (v) => linkDialog.update({ repo: v })}
          />
        {:else}
          <Input
            label="リポジトリ"
            placeholder="owner/name"
            focusOnMount
            {onkeydown}
            bind:value={() => s.repo, (v) => linkDialog.update({ repo: v })}
          />
        {/if}
      </div>
      <div class="field">
        <span>番号</span>
        <Input
          label="番号"
          focusOnMount={s.repos.length > 0}
          {onkeydown}
          bind:value={() => s.number, (v) => linkDialog.update({ number: v })}
        />
      </div>
      {#if s.repos.length === 0}
        <Checkbox
          label="プロジェクトのリポジトリにも追加する"
          bind:checked={() => s.addToProject, (v) => linkDialog.update({ addToProject: v })}
        />
      {/if}
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
      {#if s.asking}
        <p class="muted">{askingText}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={linkDialog.close}>キャンセル</Button>
      <Button variant="primary" disabled={s.asking} onclick={() => void linkDialog.submit()}
        >リンク</Button
      >
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
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .muted {
    color: var(--muted);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
