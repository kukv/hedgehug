<script lang="ts">
  import { type DetailView, readOnlyNote, saveAsTemplateLabel } from "@hedgehug/core";
  import { IconButton, Menu } from "@hedgehug/ui";
  import SaveAsTemplateDialog from "./SaveAsTemplateDialog.svelte";

  // 詳細パネルのメニュー(02「上から順の構成」)。B3 は枠と「テンプレートとして保存…」だけを作り、ほかの項目は A4 が足す(B3-17)。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  let open = $state(false);
  let x = $state(0);
  let y = $state(0);
  let saving = $state(false);

  function show(event: MouseEvent) {
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    x = r.left;
    y = r.bottom + 4;
    open = true;
  }

  function select(id: string) {
    open = false;
    if (id === "save-as-template") saving = true;
  }
</script>

<IconButton label="メニュー" size="sm" onclick={show}>⋯</IconButton>
<Menu
  {open}
  {x}
  {y}
  items={[
    {
      id: "save-as-template",
      label: saveAsTemplateLabel,
      disabled: v.readOnly,
      hint: v.readOnly ? readOnlyNote : undefined,
    },
  ]}
  onselect={select}
  onclose={() => (open = false)}
/>
{#if saving}
  <SaveAsTemplateDialog {v} onclose={() => (saving = false)} />
{/if}
