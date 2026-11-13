<script lang="ts">
  import { changesHeading, replacementsHeading } from "@hedgehug/core";
  import { Button, Dialog, Select } from "@hedgehug/ui";
  import { itemTransfer } from "../../wireActions";

  // 「別のワークスペースへ移動…」のダイアログ(02「別のワークスペースと別の Store への移動」)。開くのは
  // itemTransfer.open(item)。候補・予告・選んだ値・送信は core の itemTransfer が持つ。送っているあいだは選び直せない
  // (モデルが受けないので、セレクトも押せなくする)。
  const dialog = itemTransfer.state;
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="別のワークスペースへ移動" wide onclose={itemTransfer.cancel}>
    <div class="form">
      {#if !s.loading}
        <p class="intro">{s.intro}</p>
      {/if}
      <div class="field">
        <span>移動先のワークスペース</span>
        <Select
          label="移動先のワークスペース"
          options={s.workspaces}
          value={s.form.workspace}
          disabled={s.busy}
          onchange={(v) => void itemTransfer.setWorkspace(v)}
        />
      </div>
      <div class="field">
        <span>プロジェクト</span>
        <Select
          label="プロジェクト"
          options={s.projects}
          value={s.form.project}
          disabled={s.busy}
          onchange={(v) => void itemTransfer.setProject(v)}
        />
      </div>
      {#if s.parents !== null}
        <div class="field">
          <span>親の Epic</span>
          {#if s.noEpic}
            <p class="form-error">{s.noEpic}</p>
          {:else}
            <Select
              label="親の Epic"
              options={s.parents}
              value={s.form.parent}
              disabled={s.busy}
              onchange={(v) => itemTransfer.setParent(v)}
            />
          {/if}
        </div>
      {/if}
      {#if !s.loading}
        <h3>{changesHeading}</h3>
        <ul class="changes">
          {#each s.changes as line (line)}
            <li>{line}</li>
          {/each}
        </ul>
        {#if s.replacements.length > 0}
          <h3>{replacementsHeading}</h3>
          {#each s.replacements as row (row.status)}
            <div class="replacement">
              <span>{row.label}</span>
              <Select
                label="{row.name} の置き換え先"
                options={row.options}
                value={row.value}
                disabled={s.busy}
                onchange={(v) => itemTransfer.setReplacement(row.status, v)}
              />
            </div>
          {/each}
        {/if}
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={itemTransfer.cancel}>キャンセル</Button>
      <Button variant="primary" disabled={!s.canSubmit} onclick={() => void itemTransfer.submit()}>
        移動
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
  .intro,
  h3 {
    margin: 0;
  }
  h3 {
    font-size: 13px;
  }
  .field,
  .replacement {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span,
  .replacement > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .changes {
    margin: 0;
    padding-left: 20px;
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
