<script lang="ts">
  import { type AuthMethod, askingText, methodChoices } from "@hedgehug/core";
  import { Button, ConfirmDialog, Dialog, Input, Select } from "@hedgehug/ui";
  import { accountForm } from "../../accounts";

  // 「アカウントを追加」「アカウントを編集」(07「登録」「編集」)と「アカウントを変更」の確認。
  // 確認を出しているあいだは、編集のダイアログを隠す(D2 画面の計画の決定 S16)。
  const dialog = accountForm.state;

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void accountForm.submit();
  }
</script>

{#if $dialog?.confirm}
  <ConfirmDialog
    title="アカウントを変更"
    lines={$dialog.confirm}
    ok="変更"
    danger
    onok={() => void accountForm.confirm()}
    oncancel={accountForm.cancelConfirm}
  />
{:else if $dialog}
  {@const s = $dialog}
  <Dialog
    open
    title={s.original ? "アカウントを編集" : "アカウントを追加"}
    onclose={accountForm.close}
  >
    <div class="form">
      <div class="field">
        <span>表示名</span>
        <Input
          label="表示名"
          placeholder="work"
          focusOnMount
          {onkeydown}
          bind:value={() => s.form.name, (v) => accountForm.update({ name: v })}
        />
      </div>
      <div class="field">
        <span>GitHub のログイン名</span>
        <Input
          label="GitHub のログイン名"
          {onkeydown}
          bind:value={() => s.form.login, (v) => accountForm.update({ login: v })}
        />
        {#if s.loginError}
          <p class="form-error" role="alert">{s.loginError}</p>
        {/if}
      </div>
      <div class="field">
        <span>認証方法</span>
        <Select
          label="認証方法"
          options={methodChoices}
          bind:value={() => s.form.method, (v) => accountForm.update({ method: v as AuthMethod })}
        />
      </div>
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
      {#if s.asking}
        <p class="muted">{askingText}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={accountForm.close}>キャンセル</Button>
      <Button variant="primary" disabled={s.asking} onclick={() => void accountForm.submit()}>
        {s.original ? "保存" : "追加"}
      </Button>
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
