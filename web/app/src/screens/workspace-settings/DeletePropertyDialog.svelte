<script lang="ts">
  import { Button, Dialog } from "@hedgehug/ui";

  // プロパティの削除の確認(03「削除」)。本文は core の deletePropertyBody が作る。
  interface Props {
    body: string;
    onconfirm: () => void;
    onclose: () => void;
  }

  let { body, onconfirm, onclose }: Props = $props();
</script>

<Dialog open title="プロパティを削除" {onclose}>
  <p class="text">{body}</p>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="danger" onclick={onconfirm}>削除</Button>
  {/snippet}
</Dialog>

<style>
  .text {
    margin: 0;
  }
  .grow {
    flex: 1;
  }
</style>
