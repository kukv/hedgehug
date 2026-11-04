<script lang="ts">
  import { cliViewRename, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { ViewDraft, ViewEntry } from "../../core/views";
  import { core } from "../context";
  import { router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { carryDraft } from "./dialogs-state.svelte";
  import { sameViewName } from "./draft";

  let {
    entry,
    carry = null,
    onclose,
  }: { entry: ViewEntry; carry?: ViewDraft | null; onclose: () => void } = $props();

  const { views } = core();
  const uid = $props.id();
  const all = useSubscribe(views, () => views.list());
  const from = $derived(entry.view.name);

  // null until the user types: the field starts with the name it has.
  let typed = $state<string | null>(null);
  const name = $derived(typed ?? from);
  let touched = $state(false);
  let busy = $state(false);
  let failure = $state<string | null>(null);

  // Another view of the Store with that name (the same name with a different case is this view's
  // own to change, which the server decides).
  const taken = $derived(
    all.current.find(
      (e) => e.store === entry.store && e.view.name !== from && sameViewName(e.view.name, name),
    ),
  );
  const problem = $derived(
    taken ? `${taken.view.name} というビューが ${entry.store} にあります` : null,
  );
  const unchanged = $derived(name === from);
  const invalid = $derived(name === "" || unchanged || problem !== null);
  const shown = $derived(name === "" && !touched ? null : problem);

  const cli = $derived(
    cliViewRename(from, name === "" ? new Placeholder("新しい名前") : name, entry.store),
  );

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    touched = true;
    if (busy || invalid) return;
    busy = true;
    failure = null;
    try {
      const renamed = await views.rename(entry.store, from, name);
      // The screen of the old name has nothing to show now: it goes to the new one.
      const r = router.route;
      if (r.name === "view" && r.store === entry.store && r.view === from) {
        // What was not saved goes with the view to its new name.
        if (carry) carryDraft(entry.store, renamed.name, carry);
        router.go(router.href({ name: "view", store: entry.store, view: renamed.name }), {
          replace: true,
        });
      }
      toast({ message: `ビュー ${from} を ${renamed.name} に改名しました` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    const t = e.target as HTMLInputElement;
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && t.type === "text") {
      e.preventDefault();
      void submit();
    }
  }
</script>

<Dialog open title="ビュー {from} を改名" width={560} {cli} cliDisabled={invalid} onclose={close}>
  <div class="form" role="presentation" {onkeydown}>
    <div class="field">
      <label class="lbl" for="{uid}-name">新しい名前</label>
      <input
        id="{uid}-name"
        type="text"
        autocomplete="off"
        data-autofocus
        value={name}
        oninput={(e) => (typed = e.currentTarget.value)}
        aria-invalid={shown ? "true" : undefined}
        aria-describedby="{uid}-msg"
      >
      <span id="{uid}-msg" class="msg" class:bad={shown}>
        {shown ?? `${entry.store} の views/${from}.yaml を、新しい名前のファイルに移します`}
      </span>
    </div>
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {busy ? "改名中…" : "改名"}
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
  input[type="text"] {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  input[aria-invalid="true"] {
    border: 1.5px solid var(--danger);
  }
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.55;
  }
  .bad {
    color: var(--danger);
  }
</style>
