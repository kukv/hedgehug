<script lang="ts">
  import { notCarriedHeading, replacementSelectLabel, replacementsHeading } from "@hedgehug/core";
  import { Button, Dialog, Select } from "@hedgehug/ui";
  import { projectTransfer } from "../../wireActions";

  // 「プロジェクトを移動」の確認ダイアログ(02「名前の変更・移動・削除」の移動)。開くのは設定タブの
  // 「別のワークスペースへ移動」の区画の「移動」。予告・置き換えの選択・送信・失敗のトースト・移せたあとの移り先は
  // core の projectTransfer と wireActions が持つ。送っているあいだは選び直せない(モデルが受けないので、セレクトも
  // 押せなくする)。
  const dialog = projectTransfer.dialog;
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title={s.title} wide onclose={projectTransfer.cancel}>
    <div class="form">
      <p class="intro">{s.intro}</p>
      {#if s.notCarried.length > 0}
        <h3>{notCarriedHeading}</h3>
        <ul class="lines">
          {#each s.notCarried as line (line)}
            <li>{line}</li>
          {/each}
        </ul>
      {/if}
      {#if s.replacements.length > 0}
        <h3>{replacementsHeading}</h3>
        {#each s.replacements as row (row.status)}
          <div class="replacement">
            <span>{row.label}</span>
            <Select
              label={replacementSelectLabel(row.name)}
              options={row.options}
              value={row.value}
              disabled={s.busy}
              onchange={(v) => projectTransfer.setChoice(row.status, v)}
            />
          </div>
        {/each}
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={projectTransfer.cancel}>キャンセル</Button>
      <Button variant="primary" disabled={s.busy} onclick={() => void projectTransfer.submit()}>
        {s.ok}
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
  .replacement {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .replacement > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .lines {
    margin: 0;
    padding-left: 20px;
    color: var(--muted);
    font-size: 12.5px;
  }
  .grow {
    flex: 1;
  }
</style>
