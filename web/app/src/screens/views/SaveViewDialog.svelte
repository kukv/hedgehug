<script lang="ts">
  import { get, saveSummary, viewContent } from "@hedgehug/core";
  import { Button, Dialog, Input } from "@hedgehug/ui";
  import { search } from "../../search";
  import { saveView } from "../../views";

  // 「ビューとして保存」のダイアログ(E3b-10)。確認の表示は、開いた時点の検索の内容。断りは入力欄の下に出す。
  const dialog = saveView.state;

  let name = $state("");
  // 確認の表示は、開いたときの内容で固定する(開いているあいだに検索の内容が変わっても、保存する内容と食い違わせない)。
  let summary = $state(saveSummary(viewContent(get(search.state))));
  let wasOpen = false;
  $effect(() => {
    const open = $dialog !== null;
    if (open && !wasOpen) {
      name = "";
      summary = saveSummary(viewContent(get(search.state)));
    }
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
      <p class="summary">
        レイアウト: {summary.layout} ・ 条件:
        {#if summary.query !== null}
          <code>{summary.query}</code>
        {:else}
          (なし)
        {/if}
      </p>
      {#if $dialog.error}
        <p class="form-error" role="alert">{$dialog.error}</p>
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
