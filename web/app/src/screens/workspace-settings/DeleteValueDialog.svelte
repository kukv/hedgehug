<script lang="ts">
  import type { DeleteDialogModel } from "@hedgehug/core";
  import { Button, Dialog, Select } from "@hedgehug/ui";

  // 値の削除の確認(03「削除」)。本文は core の deleteDialog が作る。
  interface Props {
    model: DeleteDialogModel;
    onconfirm: (replacement: string) => void;
    onclose: () => void;
  }

  let { model, onconfirm, onclose }: Props = $props();

  // 開いたときの初期値だけを使う(ダイアログは削除のたびに作り直す)。
  // svelte-ignore state_referenced_locally
  let replacement = $state(model.initialReplacement ?? "");
  const replacementName = $derived(
    model.replacements?.find((r) => r.value === replacement)?.label ?? "",
  );
</script>

<Dialog open title={model.title} {onclose}>
  <p class="text">{model.body}</p>
  {#if model.replacements}
    <Select label="代わりの値" bind:value={replacement} options={model.replacements} />
  {/if}
  {#if model.replaceNote}
    <p class="text">{model.replaceNote(replacementName)}</p>
  {/if}
  {#if model.statusNote}
    <p class="text">{model.statusNote}</p>
  {/if}
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="danger" onclick={() => onconfirm(replacement)}>削除</Button>
  {/snippet}
</Dialog>

<style>
  .text {
    margin: 0;
  }
  .grow {
    flex: 1;
  }
</style>
