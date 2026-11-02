<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliWorkspaceRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { currentEntry, removeBlocker } from "../../core/workspace";
  import { core } from "../context";
  import { hrefOf, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { runUndo } from "../undo";

  let { entry, onclose }: { entry: WorkspaceEntry; onclose: () => void } = $props();

  const c = core();
  const { api, stores } = c;
  const all = useSubscribe(stores, () => stores.workspaces());
  // The counts move while the dialog is open: read the workspace as it is now.
  const w = $derived(currentEntry(all.current, entry));
  // A task added since the dialog opened keeps the workspace (the server answers 409 if this is late).
  const blocker = $derived(removeBlocker(w));

  let busy = $state(false);
  let failure = $state<string | null>(null);

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy || blocker !== null) return;
    busy = true;
    failure = null;
    const gone = { ...w };
    try {
      await unwrap(
        api.DELETE("/api/workspaces/{id}", {
          params: { path: { id: gone.ulid }, query: { store: gone.store } },
        }),
      );
      await stores.reload().catch(() => undefined);
      // The screen of the workspace that is gone has nothing to show.
      const r = router.route;
      if (r.name === "workspace" && r.store === gone.store && r.ulid === gone.ulid) {
        router.go(hrefOf({ name: "home" }));
      }
      toast({
        message: `ワークスペース ${gone.name} を削除しました`,
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
  title="ワークスペース {w.name} を削除"
  width={560}
  cli={cliWorkspaceRm({ workspace: w.name, store: w.store })}
  cliDisabled={blocker !== null}
  onclose={close}
>
  <div class="body">
    {#if blocker}
      <p class="blocked" role="note">{blocker}</p>
    {:else}
      <p class="note">
        {w.name}（<span class="mono">{w.prefix}</span>）にはタスクがありません。削除すると接頭辞
        <span class="mono">{w.prefix}</span>
        は空き、ほかのワークスペースで使えるようになります。コマンドパレットの「直前の操作を取り消す」（hh
        undo）で元に戻せます。
      </p>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="danger" disabled={busy || blocker !== null} onclick={() => void submit()}>
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
  .note,
  .blocked {
    margin: 0;
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .note {
    color: var(--ink-2);
  }
  .blocked {
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--warning-weak);
    color: var(--ink);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
</style>
