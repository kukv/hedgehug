<script lang="ts">
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { renameView, renameViewTarget } from "../../views";

  // 「名前を変更…」のダイアログ(E3b-13)。断りは入力欄の下に出す。入力欄の値は、開くたびに対象のいまの名前にする。
  const dialog = renameView.state;
  const target = renameViewTarget;

  let name = $state("");
  let wasOpen = false;
  $effect(() => {
    const open = $dialog !== null;
    if (open && !wasOpen) name = $target?.name ?? "";
    wasOpen = open;
  });

  function onkeydown(event: KeyboardEvent) {
    // Enter で保存する。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void renameView.save(name);
  }
</script>

{#if $dialog}
  <Dialog open title="ビューの名前を変更" onclose={renameView.close}>
    <div class="form">
      <div class="field">
        <span>名前</span>
        <Input label="名前" focusOnMount {onkeydown} bind:value={name} />
      </div>
      {#if $dialog.error}
        <p class="form-error" role="alert">{$dialog.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={renameView.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void renameView.save(name)}>保存</Button>
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
  .grow {
    flex: 1;
  }
</style>
