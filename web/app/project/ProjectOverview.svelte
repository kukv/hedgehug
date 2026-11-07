<script lang="ts">
  import type { TaskView } from "../../core/api/client";
  import {
    checksLabel,
    fetchLine,
    fetchTimes,
    fullRepo,
    linkUrl,
    prLinks,
    prState,
    repoLabel,
    repoOf,
    reviewLabel,
    shortLabel,
    slowedText,
  } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useGitHubRepos } from "../github.svelte";
  import { useListScope } from "../list/scope.svelte";
  import { useReadOnly } from "../read-only";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import Button from "../ui/Button.svelte";
  import IdLabel from "../ui/IdLabel.svelte";

  // The overview of a project: its repositories and the pull requests of its tasks.
  let { ulid }: { ulid: string } = $props();

  const c = core();
  const ls = useListScope(() => ({ kind: "project" as const, ulid }));
  const project = $derived(ls.project);
  const readOnly = useReadOnly(c.stores, () => ls.store);
  const github = useGitHubRepos(() => ls.store);
  const shown = useSubscribe(c.cache, () => c.cache.detail(ulid));
  const cacheRev = useSubscribe(c.cache, () => ({}));

  let list = $state<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    if (!ls.ready) return;
    const h = c.cache.watchList({ q: ls.apiBase, tree: false, store: ls.store });
    list = h;
    return () => h.dispose();
  });

  const tasks = $derived.by<TaskView[]>(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => {
      const t = c.cache.task(u);
      return t ? [t] : [];
    });
  });
  const rows = $derived(tasks.flatMap((t) => prLinks(t.links).map((link) => ({ link, task: t }))));

  const repos = $derived.by(() => {
    const counts = new Map((shown.current?.repos ?? []).map((r) => [fullRepo(r), 0]));
    for (const { link } of rows) {
      const r = repoOf(link.ref);
      counts.set(r, (counts.get(r) ?? 0) + 1);
    }
    return [...counts];
  });
  const times = $derived(fetchTimes(github.current, new Set(repos.map(([r]) => r))));
  const now = $derived.by(() => {
    void github.current;
    return new Date();
  });
  const slowed = $derived(slowedText(times.slowedUntil, now));

  let adding = $state(false);
  let name = $state("");
  let busy = $state(false);

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const repo = fullRepo(name);
    if (!project || !shown.current || repo === "" || busy) return;
    busy = true;
    try {
      const had = shown.current.repos ?? [];
      await patchTask(c, project, { repos: [...had.filter((r) => fullRepo(r) !== repo), repo] });
      name = "";
      adding = false;
    } catch (err) {
      toast({ message: errorMessage(err) });
    } finally {
      busy = false;
    }
  }
</script>

<div class="overview">
  <section class="card" aria-labelledby="pr-h">
    <div class="head">
      <h2 id="pr-h">リポジトリと PR</h2>
      {#if slowed}
        <span class="slowed" role="status">{slowed}</span>
      {/if}
      {#if repos.length > 0}
        <span class="hint">{fetchLine(times, now)}</span>
      {/if}
      <span class="end">
        {#each repos as [repo, n] (repo)}
          <span class="repo">{repoLabel(repo)} · PR {n}</span>
        {/each}
        {#if readOnly.current === null}
          <Button size="sm" onclick={() => (adding = !adding)}>リポジトリを追加</Button>
        {/if}
      </span>
    </div>

    {#if adding}
      <form class="add" onsubmit={add}>
        <input aria-label="リポジトリ" placeholder="org/repo" bind:value={name} autocomplete="off">
        <Button size="sm" type="submit" variant="confirm" disabled={busy || name.trim() === ""}>
          追加
        </Button>
      </form>
    {/if}

    {#if rows.length === 0}
      <p class="empty">リンク中の PR はありません</p>
    {:else}
      <div class="scroll">
        <table>
          <thead>
            <tr>
              <th scope="col">PR</th>
              <th scope="col">タイトル</th>
              <th scope="col">状態</th>
              <th scope="col">レビュー</th>
              <th scope="col">CI</th>
              <th scope="col">タスク</th>
            </tr>
          </thead>
          <tbody>
            {#each rows as { link, task } (`${task.ulid}/${link.ref}`)}
              {@const state = prState(link)}
              {@const href = router.href({ name: "task", ulid: task.ulid })}
              <tr>
                <td class="mono">
                  <a href={linkUrl(link)} target="_blank" rel="noreferrer">
                    {shortLabel(link.label)}
                  </a>
                </td>
                <td>{link.title ?? ""}</td>
                <td><span class="state" data-tone={state.tone}>{state.text}</span></td>
                <td>{reviewLabel(link)}</td>
                <td>{checksLabel(link) || "—"}</td>
                <td class="mono">
                  <IdLabel
                    id={task.id}
                    ulid={task.ulid}
                    store={task.store}
                    {href}
                    onopen={(e) => onLinkClick(e, href)}
                  />
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
</div>

<style>
  .overview {
    padding: var(--space-16) var(--gutter);
    overflow: auto;
    height: 100%;
    box-sizing: border-box;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-10);
    padding: var(--space-16) 18px;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-12);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
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
  .slowed {
    display: inline-flex;
    align-items: center;
    height: 1.5rem;
    padding: 0 var(--space-8);
    border-radius: var(--radius-pill);
    background: var(--warning-weak);
    color: var(--warning);
    font-size: var(--fs-12);
  }
  .end {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    margin-left: auto;
  }
  .repo {
    display: inline-flex;
    align-items: center;
    height: 1.625rem;
    padding: 0 var(--space-10);
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-6);
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .add {
    display: flex;
    gap: var(--space-8);
    align-items: center;
  }
  .add input {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-10);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-6);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .empty {
    margin: 0;
    color: var(--muted);
  }
  .scroll {
    overflow-x: auto;
  }
  table {
    width: 100%;
    min-width: 47.5rem;
    border-collapse: collapse;
  }
  th {
    padding: var(--space-8) var(--space-6);
    border-bottom: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 500;
    text-align: left;
  }
  td {
    padding: 9px var(--space-6);
    border-bottom: var(--hairline) solid var(--line);
    color: var(--ink-2);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
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
  .state:empty {
    display: none;
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
</style>
