<script lang="ts">
  import { Button, Dialog, Select } from "@hedgehug/ui";
  import { itemMove } from "../../wireActions";

  // 移動の確認ダイアログ(02「同じワークスペースの中での移動」)。プロジェクトが変わり、移し先が使わないステータスの
  // アイテムがあるときだけ、「移動…」と Alt+Shift+↑↓ が出す。中身・実行は core の itemMove が持つ。
  const dialog = itemMove.moveConfirm;
  // values は、行ごとに選んでいる置き換え先。ダイアログが開くたびに、初めの値(予告の既定)へ戻す。
  // 最初の描画では values がまだ空なので、bind は row.value を読む(bind:value に undefined は渡せない)。
  let values = $state<Record<string, string>>({});
  $effect(() => {
    values = Object.fromEntries(($dialog?.rows ?? []).map((r) => [r.status, r.value]));
  });
</script>

{#if $dialog}
  <Dialog open title={$dialog.title} onclose={itemMove.cancelMove}>
    <p class="line">{$dialog.body}</p>
    <h3 class="heading">ステータスの置き換え</h3>
    {#each $dialog.rows as row (row.status)}
      <div class="row">
        <span class="name">{row.label}</span>
        <Select
          label={row.label}
          bind:value={() => values[row.status] ?? row.value, (v) => (values[row.status] = v)}
          options={row.options.map((o) => ({ value: o.id, label: o.label }))}
        />
      </div>
    {/each}
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={itemMove.cancelMove}>キャンセル</Button>
      <Button variant="primary" onclick={() => itemMove.confirmMove({ ...values })}>移動</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .line {
    margin: 0 0 var(--space-3, 12px);
  }
  .heading {
    margin: 0 0 var(--space-2, 8px);
    font-size: 1em;
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3, 12px);
    margin-bottom: var(--space-2, 8px);
  }
  .name {
    flex: 1;
  }
  .grow {
    flex: 1;
  }
</style>
