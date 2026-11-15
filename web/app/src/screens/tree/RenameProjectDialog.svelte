<script lang="ts">
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { renameProject } from "../../projectRows";

  // 「名前を変更…」のダイアログ(E3a-14)。検査と断りの文は core の renameProject(概要のタイトルの変更と同じ)。
  const dialog = renameProject.state;

  // 入力欄の値は、開くたびに今のタイトルにする。
  let title = $state("");
  let shownFor: unknown = null;
  $effect(() => {
    const project = $dialog?.project ?? null;
    if (project === shownFor) return;
    shownFor = project;
    title = project?.title ?? "";
  });

  function onkeydown(event: KeyboardEvent) {
    // Enter で保存する。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void renameProject.save(title);
  }
</script>

{#if $dialog}
  <Dialog open title="プロジェクトの名前を変更" onclose={renameProject.close}>
    <div class="form">
      <div class="field">
        <span>タイトル</span>
        <Input label="タイトル" focusOnMount {onkeydown} bind:value={title} />
      </div>
      {#if $dialog.error}
        <p class="form-error" role="alert">{$dialog.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={renameProject.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void renameProject.save(title)}>保存</Button>
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
