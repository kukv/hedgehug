<script lang="ts">
  import { ConfirmDialog, Menu } from "@hedgehug/ui";
  import { itemKind } from "../../wireActions";

  // 「種類を変更…」の、種類のメニューと確認ダイアログ(02「種類の変更」)。メニューの項目を選ぶと、まず選べる種類の
  // メニューが出て、種類を選ぶと確認ダイアログに替わる。開くのは itemKind.open(item, x, y)。
  // 中身・実行・読み直しは core の itemKind が持つ。
  const dialog = itemKind.state;

  const options = $derived(
    $dialog?.step === "menu" ? $dialog.options.map((k) => ({ id: k, label: k })) : [],
  );

  function select(id: string) {
    const kind = $dialog?.step === "menu" ? $dialog.options.find((k) => k === id) : undefined;
    if (kind) void itemKind.choose(kind);
  }
</script>

{#if $dialog?.step === "menu"}
  <Menu
    open
    x={$dialog.x}
    y={$dialog.y}
    items={options}
    onselect={select}
    onclose={itemKind.cancel}
  />
{:else if $dialog?.step === "confirm"}
  <ConfirmDialog
    title={$dialog.title}
    lines={$dialog.lines}
    ok={$dialog.ok}
    onok={() => void itemKind.confirm()}
    oncancel={itemKind.cancel}
  />
{/if}
