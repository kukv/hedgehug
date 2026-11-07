<script lang="ts">
  import type { Schemas, ShowView } from "../../core/api/client";
  import { cliPr } from "../../core/cli";
  import {
    checksLabel,
    fetchedText,
    fetchTimes,
    issueState,
    type Link,
    linkArg,
    linkUrl,
    prLinks,
    prState,
    repoOf,
    reviewLabel,
    SYNC_MODES,
    syncedFields,
    viaLabel,
  } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useGitHubRepos, useIssueFields } from "../github.svelte";
  import { useDisplayId } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import { copyText } from "../ui/clipboard";
  import IssueCreateDialog from "./IssueCreateDialog.svelte";
  import LinkDialog from "./LinkDialog.svelte";

  // The pull requests and the issues of a task. It writes links only; 「PR を作る」 copies the
  // command, as the GUI has no repository to push from.
  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache, stores } = core();
  const did = useDisplayId(stores);
  const repos = useGitHubRepos(() => show.store);
  const fields = useIssueFields(() => show.store);

  const links = $derived(show.links ?? []);
  const issues = $derived(links.filter((l) => l.type === "github_issue"));
  const rows = $derived([...prLinks(links), ...issues]);
  const fetched = $derived.by(() => {
    const names = new Set(links.map((l) => repoOf(l.ref)));
    return fetchedText(fetchTimes(repos.current, names).last, new Date());
  });

  let linking = $state(false);
  let opening = $state(false);
  let busy = $state(false);

  async function write(run: () => Promise<Schemas.TaskWrite>) {
    busy = true;
    try {
      const r = await run();
      cache.applyWrite({ task: r.task, commit: r.commit });
    } catch (e) {
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }

  const unlink = (l: Link) => {
    const which = l.type === "github_pr" ? { pr: linkArg(l) } : { issue: linkArg(l) };
    return write(() => api.unlinkTask(show.ulid, { store: show.store, ...which }));
  };

  const setMode = (l: Link, sync: Link["sync"]) =>
    write(() => api.linkTask(show.ulid, { issue: linkArg(l), sync }, { store: show.store }));

  async function copyPr() {
    const ok = await copyText(cliPr(did.current(show.store, show.id)));
    toast({ message: ok ? "hh pr のコマンドをコピーしました" : "コピーできませんでした" });
  }
</script>

<section aria-labelledby="gh-h">
  <div class="head">
    <h2 id="gh-h">GitHub</h2>
    {#if fetched}
      <span class="hint">{fetched}</span>
    {/if}
    <span class="end">
      <Button size="sm" onclick={copyPr}>PR を作成</Button>
      {#if !readonly}
        <Button size="sm" onclick={() => (opening = true)}>Issue を作成</Button>
        <Button size="sm" onclick={() => (linking = true)}>リンク</Button>
      {/if}
    </span>
  </div>

  {#each rows as l (l.ref)}
    {@const isPr = l.type === "github_pr"}
    {@const state = isPr ? prState(l) : issueState(l)}
    <div class="row" data-ref={l.ref}>
      <a class="ref" href={linkUrl(l)} target="_blank" rel="noreferrer">{l.label}</a>
      {#if l.title}
        <span class="title">{l.title}</span>
      {/if}
      {#if state.text}
        <span class="state" data-tone={state.tone}>{state.text}</span>
      {/if}
      {#if isPr}
        <span class="review">{reviewLabel(l)}</span>
        {#if checksLabel(l)}
          <span class="checks" data-checks={l.checks}
            ><span class="dot"></span>{checksLabel(l)}</span
          >
        {/if}
      {/if}
      <span class="end">
        {#if isPr}
          {#if l.branch || viaLabel(l)}
            <span class="source">{[l.branch, viaLabel(l)].filter(Boolean).join(" · ")}</span>
          {/if}
        {:else}
          <label class="mode">
            同期
            <select
              aria-label="同期: {l.label}"
              value={l.sync}
              disabled={readonly || busy}
              onchange={(e) => void setMode(l, e.currentTarget.value as Link["sync"])}
            >
              {#each SYNC_MODES as m (m.value)}
                <option value={m.value}>{m.label}</option>
              {/each}
            </select>
          </label>
          {#if syncedFields(l, fields.current)}
            <span class="source">{syncedFields(l, fields.current)}</span>
          {/if}
        {/if}
        {#if !readonly}
          <Button
            size="sm"
            variant="ghost"
            aria-label="リンクを外す: {l.label}"
            disabled={busy}
            onclick={() => void unlink(l)}
          >
            リンクを外す
          </Button>
        {/if}
      </span>
    </div>
  {:else}
    <p class="empty">リンクはありません</p>
  {/each}
</section>

{#if linking}
  <LinkDialog task={show} onclose={() => (linking = false)} />
{/if}
{#if opening}
  <IssueCreateDialog task={show} onclose={() => (opening = false)} />
{/if}

<style>
  section {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .hint {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .end {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-6);
    margin-left: auto;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-12) 14px;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-10);
  }
  .ref {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .title {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .state {
    display: inline-flex;
    align-items: center;
    height: 1.25rem;
    padding: 0 7px;
    border-radius: var(--radius-pill);
    background: var(--hover);
    color: var(--ink-2);
    font-size: var(--fs-11);
  }
  .state[data-tone="open"] {
    background: var(--success-weak);
    color: var(--success-ink);
  }
  .state[data-tone="merged"] {
    color: var(--merged);
  }
  .state[data-tone="fail"] {
    background: var(--danger-weak);
    color: var(--danger-ink);
  }
  .state[data-tone="changes"] {
    background: var(--warning-weak);
    color: var(--warning);
  }
  .review {
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .checks {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--amber);
    font-size: var(--fs-12);
  }
  .checks[data-checks="success"] {
    color: var(--success);
  }
  .checks[data-checks="failure"] {
    color: var(--danger);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: var(--radius-pill);
    background: currentColor;
  }
  .source {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    overflow-wrap: anywhere;
  }
  .mode {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .mode select {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-6);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--accent-weak);
    color: var(--accent-ink);
    font: inherit;
    font-weight: 500;
  }
  .empty {
    margin: 0;
    color: var(--muted);
  }
</style>
