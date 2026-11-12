<script lang="ts">
  import { Button, Checkbox, Dialog } from "@hedgehug/ui";
  import { itemDialog } from "../../wire";
  import CreateItemFields from "./CreateItemFields.svelte";

  // アイテムの作成ダイアログ(02「アイテムの作成」)。値と送信は core の itemDialog が持つ。
  const dialog = itemDialog.state;
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="新規作成" wide onclose={itemDialog.close}>
    <div class="form">
      <CreateItemFields {s} />
      <p class="preview">{s.preview}</p>
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <Checkbox
        label="続けて作成"
        bind:checked={() => s.form.keepOpen, (v) => void itemDialog.update({ keepOpen: v })}
      />
      <span class="grow"></span>
      <Button onclick={itemDialog.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void itemDialog.submit()}>作成</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .preview {
    margin: 0;
    color: var(--muted);
    font-size: 12.5px;
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
