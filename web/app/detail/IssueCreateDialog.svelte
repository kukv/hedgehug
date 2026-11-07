<script lang="ts">
  import { untrack } from "svelte";
  import { cliIssue, Placeholder } from "../../core/cli";
  import { repoLabel } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useDisplayId } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Opens an issue for a task and links it in push mode, as hh issue. The GUI has no repository of
  // its own, so the repository is always said; a task that names one starts with it.
  type Props = {
    task: { ulid: string; id: string; store: string; repos?: string[] | null };
    onclose: () => void;
  };
  let { task, onclose }: Props = $props();

  const { api, cache, stores } = core();
  const did = useDisplayId(stores);
  const uid = $props.id();

  const named = $derived((task.repos ?? []).map(repoLabel));
  let repo = $state(untrack(() => (named.length === 1 ? (named[0] ?? "") : "")));
  let busy = $state(false);
  let error = $state("");

  const typed = $derived(repo.trim());
  const cli = $derived(
    cliIssue(did.current(task.store, task.id), typed || new Placeholder("org/repo")),
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (typed === "" || busy) return;
    busy = true;
    error = "";
    try {
      const r = await api.createIssue(task.ulid, { repo: typed }, { store: task.store });
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({ message: ["Issue を作成しました", ...r.warnings].join(" · ") });
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title="Issue を作成" width={560} {cli} cliDisabled={typed === ""} {onclose}>
  <form id="issue-form" class="form" onsubmit={submit}>
    <label class="field">
      <span>リポジトリ</span>
      <input list="{uid}-repos" placeholder="org/repo" autocomplete="off" bind:value={repo}>
      <datalist id="{uid}-repos">
        {#each named as r (r)}
          <option value={r}></option>
        {/each}
      </datalist>
    </label>
    <p class="msg">push モードでリンクします。タイトルと本文はタスクから写します。</p>
    {#if error}
      <FormError message={error} />
    {/if}
  </form>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button variant="confirm" type="submit" form="issue-form" disabled={busy || typed === ""}>
      作る
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
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
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
</style>
