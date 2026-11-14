<script lang="ts">
  import { bulkFields, canSubmit, changeButton, explanation } from "@hedgehug/core";
  import { Button, ConfirmDialog, Dialog, Select } from "@hedgehug/ui";
  import { bulkChange } from "../../wireActions";
  import BulkChangeValue from "./BulkChangeValue.svelte";

  // 一覧で選んだ行の一括変更のダイアログと確認(F2b2-20)。選択バーの「一括変更…」が開く。
  // 項目・値・予告・実行・トースト・読み直しは core の bulkChange が持つ。
  const dialog = bulkChange.state;
  const fields = bulkFields.map((f) => ({ value: f.id, label: f.label }));
</script>

{#if $dialog?.stage === "form"}
  {@const f = $dialog}
  <Dialog open title="選択したアイテムを一括変更" wide onclose={bulkChange.cancel}>
    <div class="form">
      <p class="summary">{f.summary}</p>
      <div class="field">
        <span>項目</span>
        <Select
          label="項目"
          options={fields}
          value={f.field}
          disabled={f.busy}
          onchange={(v) => bulkChange.setField(v as typeof f.field)}
        />
      </div>
      <div class="field">
        <span>値</span>
        <BulkChangeValue form={f} />
      </div>
      <div class="explanation">
        {#each explanation(f.field) as line (line)}
          <p>{line}</p>
        {/each}
      </div>
      {#if f.error}
        <p class="form-error" role="alert">{f.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={bulkChange.cancel}>キャンセル</Button>
      <Button
        variant="primary"
        disabled={!canSubmit(f) || f.busy}
        onclick={() => void bulkChange.submit()}
      >
        {changeButton(f.field)}
      </Button>
    {/snippet}
  </Dialog>
{:else if $dialog}
  <ConfirmDialog
    title={$dialog.title}
    lines={$dialog.lines}
    ok={$dialog.ok}
    onok={() => void bulkChange.confirm()}
    oncancel={bulkChange.cancel}
  />
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .summary,
  .explanation p {
    margin: 0;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span,
  .explanation {
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
