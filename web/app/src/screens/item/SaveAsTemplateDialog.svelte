<script lang="ts">
  import { type DetailView, saveAsTemplateDialog } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input } from "@hedgehug/ui";
  import { saveAsTemplate } from "../../wire";

  // 「テンプレートとして保存」のダイアログ(03「アイテムからテンプレートを作る」)。
  interface Props {
    v: DetailView;
    onclose: () => void;
  }

  let { v, onclose }: Props = $props();

  // 初期値は、開いたときのアイテムから取る(開いている間にアイテムが変わっても、入力は消さない)。
  const initial = () => saveAsTemplateDialog(v);
  const model = initial();
  let name = $state(model.name);
  let title = $state(model.title);
  let structure = $state(true);
  let error = $state("");
  let busy = $state(false);

  async function save() {
    if (busy) return;
    busy = true;
    error = "";
    try {
      if (await saveAsTemplate.save(v, { name, title, structure }, (m) => (error = m))) onclose();
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title="テンプレートとして保存" {onclose}>
  <div class="form">
    <Input label="名前" bind:value={name} focusOnMount />
    <Input label="タイトルの書き出し" bind:value={title} />
    {#if model.showStructure}
      <Checkbox label="子と Step を含める" bind:checked={structure} />
    {/if}
    {#if error}
      <p class="form-error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={() => void save()}>保存</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
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
