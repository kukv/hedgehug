<script lang="ts">
  import { repoLabel } from "../../core/github";
  import { useGitHubRepos } from "../github.svelte";

  // The repository and the label of hh gh import (the label is optional: without it, every open issue); the repositories the Store reads are offered.
  type Props = { store: string; form: { repo: string; label: string } };
  let { store, form }: Props = $props();

  const uid = $props.id();
  const repos = useGitHubRepos(() => store);
</script>

<label class="field">
  <span>リポジトリ</span>
  <input list="{uid}-repos" placeholder="org/repo" autocomplete="off" bind:value={form.repo}>
  <datalist id="{uid}-repos">
    {#each repos.current as r (r.repo)}
      <option value={repoLabel(r.repo)}></option>
    {/each}
  </datalist>
</label>
<label class="field">
  <span>ラベル</span>
  <input autocomplete="off" placeholder="なし（開いている Issue すべて）" bind:value={form.label}>
</label>

<style>
  .field {
    display: flex;
    flex: 1 1 10rem;
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
