<script lang="ts">
  import { cliViewRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { ViewEntry } from "../../core/views";
  import { core } from "../context";
  import { router } from "../router";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { runUndo } from "../undo";

  let { entry, onclose }: { entry: ViewEntry; onclose: () => void } = $props();

  const c = core();
  const { views } = c;

  let busy = $state(false);
  let failure = $state<string | null>(null);

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy) return;
    busy = true;
    failure = null;
    const gone = { store: entry.store, name: entry.view.name };
    try {
      await views.remove(gone.store, gone.name);
      // The screen of the view that is gone has nothing to show.
      const r = router.route;
      if (r.name === "view" && r.store === gone.store && r.view === gone.name) {
        router.go(router.href({ name: "home" }));
      }
      toast({
        message: `ビュー ${gone.name} を削除しました`,
        undo: () => runUndo(c, gone.store),
      });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open
  title="ビュー {entry.view.name} を削除"
  width={560}
  cli={cliViewRm(entry.view.name, entry.store)}
  onclose={close}
>
  <div class="body">
    <p class="note">
      {entry.store}
      の views/{entry.view.name}.yaml
      を消します。ビューが指していたタスクは変わりません。コマンドパレットの「直前の操作を取り消す」（hh
      undo）で元に戻せます。
    </p>
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="danger" disabled={busy} onclick={() => void submit()}>
      {busy ? "削除中…" : "削除"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .note {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
</style>
