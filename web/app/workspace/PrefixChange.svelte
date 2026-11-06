<script lang="ts">
  import { cliWorkspaceSetPrefix, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { WorkspaceEntry } from "../../core/stores";
  import { currentEntry, prefixProblem } from "../../core/workspace";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  let { entry, onclose }: { entry: WorkspaceEntry; onclose: () => void } = $props();

  const { api, cache, stores } = core();
  const uid = $props.id();
  const all = useSubscribe(stores, () => stores.workspaces());
  // The counts move while the dialog is open: read the workspace as it is now.
  const w = $derived(currentEntry(all.current, entry));
  // The tasks that are renumbered: the deleted ones too.
  const count = $derived(w.tasks + w.deleted);

  let prefix = $state("");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const problem = $derived(
    prefix === w.prefix
      ? "今と同じ接頭辞です"
      : prefixProblem(prefix, w.store, all.current, w.ulid),
  );
  // An empty field is not scolded.
  const shown = $derived(prefix === "" ? null : problem);

  const cli = $derived(
    cliWorkspaceSetPrefix({
      workspace: w.name,
      prefix: prefix === "" ? new Placeholder("接頭辞") : prefix,
      store: w.store,
    }),
  );

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    if (busy || problem !== null) return;
    busy = true;
    failure = null;
    const from = w.prefix;
    try {
      const r = await api.updateWorkspace(w.ulid, { prefix }, { store: w.store });
      // The commit renumbers the tasks: its tasks.changed is this tab's own.
      cache.noteOwnCommit(r.commit);
      await stores.reload().catch(() => undefined);
      const n = r.ids?.length ?? 0;
      toast({
        message: `${w.name} の接頭辞を ${from} から ${prefix} に変えました${
          n > 0 ? `（${n} 件の ID を振り直しました）` : ""
        }`,
      });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229) {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog
  open
  title="{w.name} の接頭辞を変更"
  width={560}
  {cli}
  cliDisabled={problem !== null}
  onclose={close}
>
  <div class="form">
    <div class="now">
      <span class="lbl">今</span>
      <span class="mono">{w.prefix}</span>
    </div>

    <div class="field">
      <label class="lbl" for="{uid}-prefix">新しい接頭辞</label>
      <input
        id="{uid}-prefix"
        class="mono"
        type="text"
        autocomplete="off"
        data-autofocus
        value={prefix}
        oninput={(e) => (prefix = e.currentTarget.value.toUpperCase())}
        {onkeydown}
        aria-invalid={shown ? "true" : undefined}
        aria-describedby="{uid}-msg"
      >
      <span id="{uid}-msg" class="msg" class:bad={shown}>
        {shown ?? "接頭辞は英大文字で始まる 2〜10 文字の英大文字と数字です"}
      </span>
    </div>

    {#if count > 0}
      <p class="note">
        配下の {count} 件（削除済みを含む）の ID を新しい接頭辞で振り直します（1 コミット）。
        古い ID は former_ids に残るので、検索やリンクでは引き続き引けます。
      </p>
    {:else}
      <p class="note">配下にタスクはありません。接頭辞だけが変わります。</p>
    {/if}

    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || problem !== null} onclick={() => void submit()}>
      {busy ? "変更中…" : count > 0 ? `${count} 件を振り直す` : "変更"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .now {
    display: flex;
    gap: var(--space-12);
    align-items: baseline;
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
  input {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-13);
  }
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
  .note {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
</style>
