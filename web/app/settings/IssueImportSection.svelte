<script lang="ts">
  import { core } from "../context";
  import IssueImportFields from "../states/IssueImportFields.svelte";
  import { type ImportPlace, useIssueImport } from "../states/issue-import.svelte";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";

  // Takes in the open issues with a label, under a project (the root holds only projects).
  let { store, off }: { store: string; off: string | null } = $props();

  const c = core();
  const did = useDisplayId(c.stores);
  const spaces = useSubscribe(c.stores, () =>
    c.stores.workspaces().filter((w) => w.store === store),
  );

  type Option = ImportPlace & { key: string; label: string };
  const options = $derived(
    spaces.current.flatMap((w): Option[] =>
      w.projects.map((p) => {
        const id = did.current(store, p.id);
        return { key: p.ulid, label: `${p.title} / ${id}`, parent: p.ulid, parentId: id };
      }),
    ),
  );
  let picked = $state("");
  const place = $derived(options.find((o) => o.key === picked) ?? options[0]);

  const form = useIssueImport(
    () => store,
    () => {
      const { parent, parentId } = place ?? {};
      return { parent, parentId };
    },
    () => {
      form.repo = "";
      form.label = "";
    },
  );
</script>

<div class="row">
  <IssueImportFields {store} {form} />
  <label class="field">
    <span>作成先</span>
    <select
      value={place?.key}
      onchange={(e) => {
        picked = e.currentTarget.value;
        form.forget();
      }}
    >
      {#each options as o (o.key)}
        <option value={o.key}>{o.label}</option>
      {/each}
    </select>
  </label>
  {#if form.counted}
    <Button
      variant="primary"
      disabled={form.busy || off !== null || form.counted.count === 0}
      why={off ?? undefined}
      onclick={() => void form.run(false)}
    >
      {form.counted.count}
      件を mirror で取り込む
    </Button>
  {:else}
    <Button
      variant="primary"
      disabled={form.busy || off !== null || !form.ready || !place}
      why={off ?? undefined}
      onclick={() => void form.run(true)}
    >
      件数を確認する
    </Button>
  {/if}
</div>
{#if options.length === 0}
  <p class="count">取り込み先のプロジェクトがありません。先にプロジェクトを作ってください</p>
{/if}
{#if form.counted}
  <p class="count">
    open の Issue {form.counted.count} 件 · うち {form.counted.skipped}
    件はリンク済みのため飛ばします
  </p>
{/if}
{#if form.error}
  <FormError message={form.error} />
{/if}
<CliLine command={form.cli} disabled={!form.ready} />

<style>
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-10);
  }
  .field {
    display: flex;
    flex: 1 1 12rem;
    flex-direction: column;
    gap: var(--space-6);
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  select {
    min-height: var(--control-h);
    padding: 0 var(--space-8);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-weight: 400;
  }
  .count {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
</style>
