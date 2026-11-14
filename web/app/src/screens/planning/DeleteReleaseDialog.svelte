<script lang="ts">
  import { deleteReleaseBody, type ReleaseCard, releaseDeleted } from "@hedgehug/core";
  import { Button, Dialog } from "@hedgehug/ui";
  import { releaseAction } from "../../planning";
  import { planningPort } from "../../wire";

  // リリースの削除の確認(05「削除」)。どの状態でも削除できる。断りはダイアログの中に出す。
  interface Props {
    storeId: string;
    workspaceId: string;
    card: ReleaseCard;
    // retentionDays は、その Store の削除済みの保持日数(PlanningOverview.retentionDays)。
    retentionDays: number;
    onclose: () => void;
  }

  let { storeId, workspaceId, card, retentionDays, onclose }: Props = $props();

  // 入っている数は、削除済みでないアイテムすべて(Epic・中止を含む)。
  const body = $derived(deleteReleaseBody(card.release.name, card.items.length, retentionDays));
  let error = $state("");
  let busy = $state(false);

  async function remove() {
    if (busy) return;
    busy = true;
    const id = card.release.id;
    const ok = await releaseAction(
      async () => {
        await planningPort.deleteRelease(storeId, workspaceId, id);
        return releaseDeleted;
      },
      (message) => (error = message),
    );
    busy = false;
    if (ok) onclose();
  }
</script>

<Dialog open title="リリースを削除" {onclose}>
  <p class="text">{body}</p>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="danger" disabled={busy} onclick={remove}>削除</Button>
  {/snippet}
</Dialog>

<style>
  .text {
    margin: 0;
  }
  .error {
    margin: 8px 0 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
