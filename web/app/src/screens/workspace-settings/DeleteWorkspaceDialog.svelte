<script lang="ts">
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { workspaceDelete } from "../../wireActions";

  // ワークスペースの削除の確認ダイアログ(03「削除」)。「一般」の削除ボタンとサイドバーのメニューの「削除…」が開く。
  // 中身・名前の照合・実行・トースト・ホームへ移ることは、core の workspaceDelete と wireActions が持つ。
  const dialog = workspaceDelete.state;

  let typed = $state("");

  // ダイアログが閉じたら、入力を空に戻す。
  $effect(() => {
    if ($dialog === null) typed = "";
  });

  function change(value: string) {
    typed = value;
    workspaceDelete.clearError();
  }
  function key(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void workspaceDelete.confirm(typed);
  }
</script>

{#if $dialog}
  <Dialog open title={$dialog.title} onclose={workspaceDelete.cancel}>
    {#each $dialog.lines as line, i (i)}
      <p class="line">{line}</p>
    {/each}
    <div class="field">
      <label for="ws-delete-name">{$dialog.label}</label>
      <Input
        id="ws-delete-name"
        label={$dialog.label}
        value={typed}
        oninput={change}
        onkeydown={key}
        focusOnMount
      />
      {#if $dialog.error}
        <p class="error" role="alert">{$dialog.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={workspaceDelete.cancel}>キャンセル</Button>
      <Button variant="danger" onclick={() => void workspaceDelete.confirm(typed)}
        >{$dialog.ok}</Button
      >
    {/snippet}
  </Dialog>
{/if}

<style>
  .line {
    margin: 0 0 8px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 12px;
  }
  .field > label {
    font-size: 12.5px;
    color: var(--muted);
  }
  .error {
    margin: 0;
    font-size: 12.5px;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
