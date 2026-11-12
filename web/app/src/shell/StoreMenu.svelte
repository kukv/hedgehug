<script lang="ts">
  import { readOnlyNote, type StoreNode } from "@hedgehug/core";
  import { IconButton, Menu } from "@hedgehug/ui";
  import { workspaceDialog } from "../wire";

  // サイドバーの Store のメニュー(04「Store のメニュー」)。B2 は「ワークスペースを追加」だけを置き、
  // ほかの項目はトラック C・G が足す(B2-17)。
  interface Props {
    store: StoreNode;
  }

  let { store }: Props = $props();

  let open = $state(false);
  let x = $state(0);
  let y = $state(0);

  function show(event: MouseEvent) {
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    x = r.left;
    y = r.bottom + 4;
    open = true;
  }

  function select(id: string) {
    open = false;
    if (id === "add-workspace") workspaceDialog.open(store.id);
  }
</script>

<IconButton label="Store のメニュー" size="xs" onclick={show}>⋯</IconButton>
<Menu
  {open}
  {x}
  {y}
  items={[
    {
      id: "add-workspace",
      label: "ワークスペースを追加",
      disabled: store.readOnly,
      hint: store.readOnly ? readOnlyNote : undefined,
    },
  ]}
  onselect={select}
  onclose={() => (open = false)}
/>
