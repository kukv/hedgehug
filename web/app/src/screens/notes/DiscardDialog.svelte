<script lang="ts">
  import { ConfirmDialog } from "@hedgehug/ui";
  import { drafts } from "../../drafts";

  // 「入力を破棄」の確認(02「説明」)。出す条件と答えの扱いは core の drafts が持つ。「キャンセル」は
  // 入力欄に戻る(確認を呼んだ側が、false の答えで入力欄にフォーカスを戻す)。
  const prompt = drafts.prompt;
</script>

{#if $prompt}
  <ConfirmDialog
    title={$prompt.title}
    lines={$prompt.lines}
    ok={$prompt.ok}
    danger
    onok={() => drafts.answer(true)}
    oncancel={() => drafts.answer(false)}
  />
{/if}
