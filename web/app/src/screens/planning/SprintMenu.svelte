<script lang="ts">
  import { menuAt, type Sprint, sprintMenu } from "@hedgehug/core";
  import { IconButton, Menu } from "@hedgehug/ui";

  // スプリントのカードの「⋯」とメニュー。選んだ項目の動きは親が渡す。
  interface Props {
    sprint: Sprint;
    readOnly: boolean;
    onedit: () => void;
    onburndown: () => void;
    ondelete: () => void;
  }

  let { sprint, readOnly, onedit, onburndown, ondelete }: Props = $props();

  let menu = $state<{ x: number; y: number } | null>(null);

  function select(id: string) {
    menu = null;
    if (id === "edit") onedit();
    else if (id === "burndown") onburndown();
    else ondelete();
  }
</script>

<IconButton
  label="{sprint.name} のメニュー"
  size="sm"
  onclick={(e) => (menu = menuAt((e.currentTarget as HTMLElement).getBoundingClientRect()))}
  >⋯</IconButton
>
<Menu
  open={menu !== null}
  x={menu?.x ?? 0}
  y={menu?.y ?? 0}
  items={sprintMenu(sprint, readOnly)}
  onselect={select}
  onclose={() => (menu = null)}
/>
