<script lang="ts">
  import { saveSummary } from "@hedgehug/core";
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { saveView, saveViewContent } from "../../views";

  // 「ビューとして保存」のダイアログ(E3b-10)。確認の表示は、openSaveView に渡した保存する内容。断りは入力欄の下に出す。
  const dialog = saveView.state;
  const content = saveViewContent;

  const summary = $derived($content ? saveSummary($content) : null);
  let name = $state("");
  // 名前は、開くたびに空にする。描く前に空にする($effect.pre)ので、前の名前が一瞬も出ない。
  let wasOpen = false;
  $effect.pre(() => {
    const open = $dialog !== null;
    if (open && !wasOpen) name = "";
    wasOpen = open;
  });

  function onkeydown(event: KeyboardEvent) {
    // Enter で保存する。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void saveView.save(name);
  }
</script>

{#if $dialog}
  <Dialog open title="ビューとして保存" onclose={saveView.close}>
    <div class="form">
      <div class="field">
        <span>名前</span>
        <Input label="名前" focusOnMount {onkeydown} bind:value={name} />
      </div>
      {#if $dialog.error}
        <p class="form-error" role="alert">{$dialog.error}</p>
      {/if}
      {#if summary}
        <p class="summary">
          レイアウト: {summary.layout} ・ 条件:
          {#if summary.query !== null}
            <code>{summary.query}</code>
          {:else}
            (なし)
          {/if}
        </p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={saveView.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void saveView.save(name)}>保存</Button>
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
  .summary {
    margin: 0;
    font-size: 13px;
    color: var(--muted);
  }
  .summary code {
    font-family: var(--font-mono);
    color: var(--text, inherit);
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
