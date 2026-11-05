<script lang="ts">
  import { type Schemas, unwrap } from "../../core/api/client";
  import { cliGhImport, Placeholder } from "../../core/cli";
  import { type ImportTarget, repoLabel } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useGitHubRepos } from "../github.svelte";
  import { useDisplayId } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Makes a task in mirror mode for each open issue with a label, under the project. The count
  // comes first (dry_run): what the button says is what it takes in.

  let { target, onclose }: { target: ImportTarget; onclose: () => void } = $props();

  const c = core();
  const did = useDisplayId(c.stores);
  const repos = useGitHubRepos(() => target.store);

  let repo = $state("");
  let label = $state("");
  let counted = $state<Schemas["ImportResult"] | null>(null);
  let busy = $state(false);
  let error = $state("");

  const ready = $derived(repo.trim() !== "" && label.trim() !== "");
  const cli = $derived(
    cliGhImport({
      repo: repo.trim() || new Placeholder("org/repo"),
      label: label.trim() || new Placeholder("ラベル"),
      parent: did.current(target.store, target.parentId),
    }),
  );

  const send = (dry_run: boolean) =>
    unwrap(
      c.api.POST("/api/github/import", {
        params: { query: { store: target.store } },
        body: { repo: repo.trim(), label: label.trim(), parent: target.parent, dry_run },
      }),
    );

  async function run(dry: boolean) {
    if (!ready || busy) return;
    busy = true;
    error = "";
    try {
      const r = await send(dry);
      if (dry) {
        counted = r;
      } else {
        toast({ message: `${r.created.length} 件を取り込みました` });
        onclose();
      }
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title="Issue から取り込む" width={560} {cli} cliDisabled={!ready} {onclose}>
  <div class="form">
    <label class="field">
      <span>リポジトリ</span>
      <input
        list="import-repos"
        placeholder="org/repo"
        autocomplete="off"
        bind:value={repo}
        oninput={() => (counted = null)}
      >
      <datalist id="import-repos">
        {#each repos.current as r (r.repo)}
          <option value={repoLabel(r.repo)}></option>
        {/each}
      </datalist>
    </label>
    <label class="field">
      <span>ラベル</span>
      <input autocomplete="off" bind:value={label} oninput={() => (counted = null)}>
    </label>
    {#if counted}
      <p class="count">
        open の Issue {counted.count} 件 · うち {counted.skipped} 件はリンク済みのため飛ばします
      </p>
    {/if}
    {#if error}
      <FormError message={error} />
    {/if}
  </div>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    {#if counted}
      <Button
        variant="confirm"
        disabled={busy || counted.count === 0}
        onclick={() => void run(false)}
      >
        {counted.count}
        件を mirror で取り込む
      </Button>
    {:else}
      <Button variant="confirm" disabled={busy || !ready} onclick={() => void run(true)}>
        件数を確かめる
      </Button>
    {/if}
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
  .count {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
</style>
