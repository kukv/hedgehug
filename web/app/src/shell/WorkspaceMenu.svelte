<script lang="ts">
  import { readOnlyNote, type StoreNode, type WorkspaceNode } from "@hedgehug/core";
  import { IconButton, Menu } from "@hedgehug/ui";
  import { projectDialog } from "../wire";

  // サイドバーのワークスペースのメニュー(04「ワークスペースのメニュー」)。A2 は「プロジェクトを作成」だけを置き、
  // ほかの項目はトラック B2 が足す(トラック B の計画の未決 1)。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
  }

  let { store, workspace }: Props = $props();

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
    if (id === "create-project") void projectDialog.open(workspace.id);
  }
</script>

<IconButton label="ワークスペースのメニュー" size="xs" onclick={show}>⋯</IconButton>
<Menu
  {open}
  {x}
  {y}
  items={[
    {
      id: "create-project",
      label: "プロジェクトを作成",
      disabled: store.readOnly,
      hint: store.readOnly ? readOnlyNote : undefined,
    },
  ]}
  onselect={select}
  onclose={() => (open = false)}
/>
