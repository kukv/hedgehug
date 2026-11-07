<script lang="ts">
  import { cliReleaseRm, cliSprintRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import type { PlanDialog } from "./dialogs-state.svelte";
  import { plansWritten } from "./plans.svelte";

  type Props = {
    dialog: Extract<PlanDialog, { kind: "sprint-rm" | "release-rm" }>;
    onclose: () => void;
  };
  let { dialog, onclose }: Props = $props();

  const c = core();
  const store = $derived(dialog.entry.store);
  const noun = $derived(dialog.kind === "sprint-rm" ? "スプリント" : "リリース");
  const plan = $derived(dialog.kind === "sprint-rm" ? dialog.sprint : dialog.release);
  const cli = $derived(
    dialog.kind === "sprint-rm"
      ? cliSprintRm(dialog.sprint.name, store)
      : cliReleaseRm(dialog.release.name, store),
  );

  let busy = $state(false);
  let failure = $state<string | null>(null);

  async function remove() {
    busy = true;
    failure = null;
    try {
      if (dialog.kind === "sprint-rm") {
        await c.api.removeSprint(plan.ulid, { store });
      } else {
        await c.api.removeRelease(plan.ulid, { store });
      }
      await c.stores.reload().catch(() => undefined);
      plansWritten();
      toast({ message: `${noun} ${plan.name} を削除しました` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title={`${noun} ${plan.name} を削除`} width={560} {cli} {onclose}>
  <p>
    入っているタスク
    {plan.tasks}
    件から外して削除します。タスクは消えません。コマンドパレットの「直前の操作を取り消す」（hh
    undo）で元に戻せます。
  </p>
  {#if failure}
    <FormError message={failure} />
  {/if}
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button variant="danger" disabled={busy} onclick={() => void remove()}>削除</Button>
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
    line-height: 1.7;
  }
</style>
