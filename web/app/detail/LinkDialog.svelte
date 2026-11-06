<script lang="ts">
  import { cliLink, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useDisplayId } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Links a pull request or an issue by its number. An issue gets the default mode of the Store:
  // the mode of a link is changed on its row after.
  type Props = {
    task: { ulid: string; id: string; store: string };
    onclose: () => void;
  };
  let { task, onclose }: Props = $props();

  const { api, cache, stores } = core();
  const did = useDisplayId(stores);
  const id = $derived(did.current(task.store, task.id));

  let kind = $state<"pr" | "issue">("pr");
  let ref = $state("");
  let busy = $state(false);
  let error = $state("");

  const typed = $derived(ref.trim());
  const cli = $derived(cliLink({ id, kind, ref: typed || new Placeholder("org/repo#123") }));

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (typed === "" || busy) return;
    busy = true;
    error = "";
    try {
      const r = await api.linkTask(task.ulid, kind === "pr" ? { pr: typed } : { issue: typed }, {
        store: task.store,
      });
      cache.applyWrite({ task: r.task, commit: r.commit });
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title="{id} にリンク" width={560} {cli} cliDisabled={typed === ""} {onclose}>
  <form id="link-form" class="form" onsubmit={submit}>
    <fieldset class="seg" aria-label="リンクするもの">
      <button
        type="button"
        class:on={kind === "pr"}
        aria-pressed={kind === "pr"}
        onclick={() => (kind = "pr")}
      >
        PR
      </button>
      <button
        type="button"
        class:on={kind === "issue"}
        aria-pressed={kind === "issue"}
        onclick={() => (kind = "issue")}
      >
        Issue
      </button>
    </fieldset>
    <label class="field">
      <span>{kind === "pr" ? "PR" : "Issue"}</span>
      <input bind:value={ref} placeholder="org/repo#123" autocomplete="off" spellcheck="false">
    </label>
    {#if error}
      <FormError message={error} />
    {/if}
  </form>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button type="submit" form="link-form" variant="confirm" disabled={busy || typed === ""}>
      リンクする
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .seg {
    display: inline-flex;
    align-self: flex-start;
    margin: 0;
    border: 0;
    padding: 3px;
    border-radius: var(--radius-8);
    background: var(--hover);
  }
  .seg button {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-10);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .seg button.on {
    background: var(--surface);
    color: var(--ink);
    font-weight: 600;
    box-shadow: var(--shadow-popover);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .field input {
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    font-weight: 400;
  }
</style>
