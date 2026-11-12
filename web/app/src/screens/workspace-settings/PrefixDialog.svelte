<script lang="ts">
  import { Button, Dialog } from "@hedgehug/ui";

  // 接頭辞の変更の確認(03「接頭辞」)。本文は core の prefixConfirm が作る。
  interface Props {
    text: string;
    onconfirm: () => void;
    onclose: () => void;
  }

  let { text, onconfirm, onclose }: Props = $props();
</script>

<Dialog open title="接頭辞を変更" {onclose}>
  <p class="text">{text}</p>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" onclick={onconfirm}>変更</Button>
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
