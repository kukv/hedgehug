<script lang="ts">
  import Input from "./Input.svelte";
  import Menu from "./Menu.svelte";

  // テスト用(menuFocus.test.ts)。「移動…」と同じ形で、head の検索欄が出たときにフォーカスを取る。
  interface Props {
    open: boolean;
  }

  let { open }: Props = $props();
</script>

<Menu
  {open}
  x={0}
  y={0}
  items={[{ id: "a", label: "候補" }]}
  onselect={() => {}}
  onclose={() => {}}
>
  {#snippet head()}
    <Input label="移動先を検索" focusOnMount />
  {/snippet}
</Menu>
