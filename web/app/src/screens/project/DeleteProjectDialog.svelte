<script lang="ts">
  import { ConfirmDialog } from "@hedgehug/ui";
  import { projectDelete } from "../../wireActions";

  // プロジェクトの削除の確認ダイアログ(02「名前の変更・移動・削除」の削除)。設定タブの「プロジェクトを削除」が開く。
  // 中身・実行・トースト・ワークスペースの概要へ移ることは、core の projectDelete と wireActions が持つ。
  const dialog = projectDelete.state;
</script>

{#if $dialog}
  <ConfirmDialog
    title={$dialog.title}
    lines={$dialog.lines}
    ok={$dialog.ok}
    danger
    onok={() => void projectDelete.confirm()}
    oncancel={projectDelete.cancel}
  />
{/if}
