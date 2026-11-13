<script lang="ts">
  import {
    canMoveWorkspace,
    get,
    readOnlyNote,
    routeOfWorkspace,
    type StoreNode,
    type WorkspaceNode,
  } from "@hedgehug/core";
  import { IconButton, Menu } from "@hedgehug/ui";
  import { catalog, navigation, projectDialog, workspaceMover } from "../wire";

  // サイドバーのワークスペースのメニュー(04「ワークスペースのメニュー」)。
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
    if (id === "settings") {
      navigation.go(routeOfWorkspace(get(catalog.stores), store.id, workspace.id, "settings"));
    }
    if (id === "up" || id === "down") void workspaceMover.move(store.id, workspace.id, id);
  }

  const move = (direction: "up" | "down") => ({
    disabled: !canMoveWorkspace(store, workspace.id, direction),
    hint: store.readOnly ? readOnlyNote : undefined,
  });
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
    { id: "settings", label: "設定を開く" },
    { id: "up", label: "上へ移動", ...move("up") },
    { id: "down", label: "下へ移動", ...move("down") },
  ]}
  onselect={select}
  onclose={() => (open = false)}
/>
