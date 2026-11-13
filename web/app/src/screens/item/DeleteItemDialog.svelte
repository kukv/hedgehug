<script lang="ts">
  import { ConfirmDialog } from "@hedgehug/ui";
  import { itemDelete } from "../../wireActions";

  // アイテムの削除の確認ダイアログ(02「削除」)。アイテムのメニューの「削除…」と Step の「×」が開く。
  // 中身・実行・トースト・読み直しは core の itemDelete が持つ。
  const dialog = itemDelete.state;
</script>

{#if $dialog}
  <ConfirmDialog
    title={$dialog.title}
    lines={$dialog.lines}
    ok={$dialog.ok}
    danger
    onok={() => void itemDelete.confirm()}
    oncancel={itemDelete.cancel}
  />
{/if}
