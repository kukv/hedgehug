<script lang="ts">
  import { workspaceTransferCancel } from "@hedgehug/core";
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { workspaceTransfer } from "../../wireActions";

  // ワークスペースの別の Store への移動の確認ダイアログ(03「別の Store へ移動」)。開くのは「一般」の区画の「移動…」。
  // 中身・名前の照合・送信・トースト・移せたあとの移り先は、core の workspaceTransfer と wireActions が持つ。
  // 削除と同じ作りだが、危険操作の見た目にはしない。
  const dialog = workspaceTransfer.dialog;

  let typed = $state("");

  // ダイアログが閉じたら、入力を空に戻す。
  $effect(() => {
    if ($dialog === null) typed = "";
  });

  function change(value: string) {
    typed = value;
    workspaceTransfer.clearError();
  }
  function key(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void workspaceTransfer.submit(typed);
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title={s.title} wide onclose={workspaceTransfer.cancel}>
    {#each s.lines as line, i (i)}
      <p class="line">{line}</p>
    {/each}
    <div class="field">
      <label for="ws-transfer-name">{s.label}</label>
      <Input
        id="ws-transfer-name"
        label={s.label}
        value={typed}
        oninput={change}
        onkeydown={key}
        focusOnMount
      />
      {#if s.error}
        <p class="error" role="alert">{s.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={workspaceTransfer.cancel}>{workspaceTransferCancel}</Button>
      <Button
        variant="primary"
        disabled={s.busy}
        onclick={() => void workspaceTransfer.submit(typed)}
        >{s.ok}</Button
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
