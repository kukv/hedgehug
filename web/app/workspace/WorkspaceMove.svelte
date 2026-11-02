<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { cliMv, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { detachedText } from "../../core/workspace";
  import { core } from "../context";
  import { workspaceKey } from "../create/context";
  import WorkspaceChoice from "../create/WorkspaceChoice.svelte";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import type { MoveTask } from "./dialogs-state.svelte";

  let { task, onclose }: { task: MoveTask; onclose: () => void } = $props();

  const { api, cache, stores } = core();
  const uid = $props.id();
  const all = useSubscribe(stores, () => stores.workspaces());

  // A task goes to another workspace of its own Store: another Store is a different move.
  const choices = $derived(all.current.filter((w) => w.store === task.store));
  const here = $derived(task.workspace.ulid);
  let picked = $state("");
  const target = $derived(
    choices.find((w) => workspaceKey(w) === picked && w.ulid !== here) ??
      choices.find((w) => w.ulid !== here),
  );

  let busy = $state(false);
  let failure = $state<string | null>(null);

  const cli = $derived(
    cliMv(task.id, { workspace: target ? target.name : new Placeholder("ワークスペース") }),
  );

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy || !target) return;
    busy = true;
    failure = null;
    const to = target;
    try {
      const r = await unwrap(
        api.POST("/api/tasks/{id}/move", {
          params: { path: { id: task.ulid }, query: { store: task.store } },
          body: { workspace: to.ulid },
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      const lost = detachedText(r.detached);
      toast({
        message: `${task.id} を ${to.name} に移しました（今の ID は ${r.task.id}）${lost ? `。${lost}` : ""}`,
        link: { label: "開く", href: router.href({ name: "task", ulid: task.ulid }) },
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
  title="{task.id} {task.title} を移動"
  width={620}
  {cli}
  cliDisabled={!target}
  onclose={close}
>
  <div class="form">
    {#if target}
      <div class="field">
        <span class="lbl">移り先のワークスペース</span>
        <WorkspaceChoice
          entries={choices}
          value={workspaceKey(target)}
          name="{uid}-to"
          label="移り先のワークスペース"
          showStore={false}
          isDisabled={(w) => w.ulid === here}
          note={(w) => (w.ulid === here ? "今のワークスペース" : "")}
          onchange={(key) => (picked = key)}
        />
      </div>

      <ul class="notes">
        <li>移り先のルートに置きます。配下のタスクも一緒に移ります</li>
        <li>ID を {target.prefix} の番号に振り直します（{task.id} は former_ids に残ります）</li>
        <li>スプリント・リリースが移り先のものでなければ外れます</li>
      </ul>
    {:else}
      <p class="none">この Store（{task.store}）にはほかのワークスペースがありません。</p>
    {/if}

    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || !target} onclick={() => void submit()}>
      {busy ? "移動中…" : "移動"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .lbl {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .notes {
    margin: 0;
    padding-left: var(--space-20);
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.8;
  }
  .none {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
  }
</style>
