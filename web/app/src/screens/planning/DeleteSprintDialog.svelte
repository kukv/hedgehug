<script lang="ts">
  import { deleteBody, type SprintCard, sprintDeleted } from "@hedgehug/core";
  import { Button, Dialog } from "@hedgehug/ui";
  import { sprintAction } from "../../planning";
  import { planningPort } from "../../wire";

  // スプリントの削除の確認(05「削除」)。断り(実行中など)はダイアログの中に出す。
  interface Props {
    storeId: string;
    workspaceId: string;
    card: SprintCard;
    // retentionDays は、その Store の削除済みの保持日数(PlanningOverview.retentionDays)。
    retentionDays: number;
    onclose: () => void;
  }

  let { storeId, workspaceId, card, retentionDays, onclose }: Props = $props();

  // 入っている数は、削除済みでないアイテムすべて(完了・中止を含む)。
  const body = $derived(deleteBody(card.sprint.name, card.items.length, retentionDays));
  let error = $state("");
  let busy = $state(false);

  async function remove() {
    if (busy) return;
    busy = true;
    const id = card.sprint.id;
    const ok = await sprintAction(
      async () => {
        await planningPort.deleteSprint(storeId, workspaceId, id);
        return sprintDeleted;
      },
      (message) => (error = message),
    );
    busy = false;
    if (ok) onclose();
  }
</script>

<Dialog open title="スプリントを削除" {onclose}>
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
