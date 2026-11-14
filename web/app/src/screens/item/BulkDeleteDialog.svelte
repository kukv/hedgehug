<script lang="ts">
  import { ConfirmDialog } from "@hedgehug/ui";
  import { bulkDelete } from "../../wireActions";

  // 一覧で選んだ行の一括削除の確認ダイアログ(F2b1-11)。選択バーの「一括削除…」が開く。
  // 中身・実行・トースト・読み直しは core の bulkDelete が持つ。
  const dialog = bulkDelete.state;
</script>

{#if $dialog}
  <ConfirmDialog
    title={$dialog.title}
    lines={$dialog.lines}
    ok={$dialog.ok}
    danger
    onok={() => void bulkDelete.confirm()}
    oncancel={bulkDelete.cancel}
  />
{/if}
