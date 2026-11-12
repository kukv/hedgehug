<script lang="ts">
  import { prefixPreviewHelp } from "@hedgehug/core";
  import { Button, Dialog, Input, Select } from "@hedgehug/ui";
  import { workspaceDialog } from "../../wire";

  // ワークスペースの作成ダイアログ(03「ワークスペースの作成」)。値と送信は core の workspaceDialog が持つ。
  const dialog = workspaceDialog.state;

  function onkeydown(event: KeyboardEvent) {
    // Enter で作成する。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void workspaceDialog.submit();
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="ワークスペースを追加" onclose={workspaceDialog.close}>
    {#if s.blocked}
      <p class="blocked">{s.blocked}</p>
    {:else}
      <div class="form">
        <div class="field">
          <span>Store</span>
          <Select
            label="Store"
            options={s.stores}
            bind:value={() => s.form.storeId, (v) => workspaceDialog.update({ storeId: v })}
          />
        </div>
        <div class="field">
          <span>名前</span>
          <Input
            label="名前"
            focusOnMount
            {onkeydown}
            bind:value={() => s.form.name, (v) => workspaceDialog.update({ name: v })}
          />
        </div>
        <div class="field">
          <span>接頭辞</span>
          <Input
            label="接頭辞"
            {onkeydown}
            bind:value={() => s.form.prefix, (v) => workspaceDialog.update({ prefix: v })}
          />
          <small>{prefixPreviewHelp}</small>
        </div>
        {#if s.error}
          <p class="form-error" role="alert">{s.error}</p>
        {/if}
      </div>
    {/if}
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={workspaceDialog.close}>{s.blocked ? "閉じる" : "キャンセル"}</Button>
      {#if !s.blocked}
        <Button variant="primary" onclick={() => void workspaceDialog.submit()}>作成</Button>
      {/if}
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
  .field > span,
  small {
    font-size: 12.5px;
    color: var(--muted);
  }
  .blocked {
    margin: 0;
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
