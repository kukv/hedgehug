<script lang="ts">
  import { CATEGORIES, CATEGORY_LABEL } from "../../core/categories";
  import { cliLs } from "../../core/cli";
  import { formatDate } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { statusColorOf } from "../../core/schema-edit";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { onLinkClick, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { baseQuery } from "../tree/filter";
  import { propText } from "../tree/marks";
  import Avatar from "../ui/Avatar.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import IdLabel from "../ui/IdLabel.svelte";
  import { countByCategory, projectsOf, rootsOf } from "./counts";
  import WorkspaceHeader from "./WorkspaceHeader.svelte";

  let { entry }: { entry: WorkspaceEntry } = $props();

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  // The effects below depend on these two only: a rename or a new count makes a new entry, and
  // that must not drop the query and start it again (the screen would flash "loading").
  const store = $derived(entry.store);
  const ulid = $derived(entry.ulid);

  // The create buttons are disabled in a Store that cannot be written (細部 35).
  const offSentence = useReadOnlySentence(c.stores, () => store);
  const off = $derived(offSentence.current);

  // The tasks of the workspace, deleted ones not included. The cache fetches the query again at
  // every tasks.changed and resync, so the counts and both lists follow the store. The ULID names
  // the workspace without the doubt a name that is the prefix of another has.
  let list = $state.raw<ReturnType<typeof c.cache.watchList> | null>(null);
  $effect(() => {
    const w = c.cache.watchList({ q: `workspace:${ulid}`, tree: false, store });
    list = w;
    return () => w.dispose();
  });

  // The categories need the schema of the store; the band waits for it.
  let schemaError = $state.raw<unknown>(null);
  $effect(() => {
    schemaError = null;
    const s = store;
    c.schema.load(s).catch((e: unknown) => {
      if (s === store) schemaError = e;
    });
  });
  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(store);
  });

  const loading = $derived.by(() => {
    cacheRev.current;
    return list === null || list.loading();
  });
  const error = $derived.by(() => {
    cacheRev.current;
    return list?.error() ?? null;
  });
  const tasks = $derived.by(() => {
    cacheRev.current;
    return (list?.ulids() ?? []).flatMap((u) => c.cache.task(u) ?? []);
  });

  const projects = $derived(projectsOf(tasks));
  const roots = $derived(rootsOf(tasks));
  const counts = $derived(schema && !loading && !error ? countByCategory(tasks, schema) : null);
  // Until the list has come, the count of the workspace is the one the store directory has.
  const total = $derived(loading || error ? entry.tasks : tasks.length);
  const statusDefOf = (status: string) => schema?.statuses?.find((s) => s.name === status);
  const statusCategory = (status: string) => statusDefOf(status)?.category;

  // Not written as scope="..." in the markup: Biome reads that name as the HTML attribute.
  const emptyProps = $derived({ scope: "workspace" as const, name: entry.name });
  const treeHref = $derived(router.href({ name: "workspace", store, ulid, tab: "tree" }));
  const projectHref = (u: string) => router.href({ name: "project", ulid: u, tab: "tree" });
  const taskHref = (u: string) => router.href({ name: "task", ulid: u });
  const today = new Date();
  const dueOf = (t: Parameters<typeof propText>[0]) => {
    const d = propText(t, "due");
    return d ? formatDate(d, today) : "—";
  };
  const percent = (p: { done: number; total: number }) =>
    p.total > 0 ? Math.round((p.done / p.total) * 100) : 0;
</script>

<WorkspaceHeader {entry} tab="overview">
  {#snippet actions()}
    <Button
      size="sm"
      aria-label="タスクを作成"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => openCreate({ kind: "task" })}
    >
      <Icon name="plus" size={14} />タスク
    </Button>
    <Button
      size="sm"
      aria-label="プロジェクトを作成"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => openCreate({ kind: "project" })}
    >
      <Icon name="folder-plus" size={14} />プロジェクト
    </Button>
  {/snippet}
</WorkspaceHeader>

<div class="overview">
  {#if error}
    <p class="msg error" role="alert">{errorMessage(error)}</p>
  {:else if !loading && tasks.length === 0}
    <Empty
      variant="empty"
      {...emptyProps}
      createOff={off ?? undefined}
      onCreate={() => openCreate({ kind: "task" })}
    />
  {:else}
    <section class="counts" aria-label="件数">
      <p class="total">{total} 件{loading ? "" : ` · プロジェクト ${projects.length}`}</p>
      <p class="note">削除済みは含めません</p>
      {#if schemaError && !schema}
        <p class="msg error" role="alert">
          スキーマを読み込めないため、カテゴリ別の件数を出せません · {errorMessage(schemaError)}
        </p>
      {:else if counts}
        <div
          class="band"
          role="img"
          aria-label={CATEGORIES.map((k) => `${CATEGORY_LABEL[k]} ${counts.byCategory[k]}`).join(
            " · ",
          )}
        >
          {#each CATEGORIES as k (k)}
            {#if counts.byCategory[k] > 0}
              <span class="seg" data-category={k} style:flex-grow={counts.byCategory[k]}></span>
            {/if}
          {/each}
        </div>
        <ul class="legend">
          {#each CATEGORIES as k (k)}
            <li>
              <StatusIcon status={k} category={k} size={12} />{CATEGORY_LABEL[k]}
              {counts.byCategory[k]}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    {#if loading}
      <p class="msg">読み込み中…</p>
    {:else}
      <section aria-labelledby="ov-projects">
        <div class="sec-head">
          <h2 id="ov-projects">プロジェクト</h2>
          <span class="sub">{projects.length} 件</span>
        </div>
        {#if projects.length === 0}
          <p class="none">プロジェクトはまだありません</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th class="w-id">ID</th>
                <th>タイトル</th>
                <th class="w-progress">進捗</th>
                <th class="w-due">期限</th>
                <th class="w-who">担当者</th>
              </tr>
            </thead>
            <tbody>
              {#each projects as p (p.ulid)}
                {@const href = projectHref(p.ulid)}
                <tr>
                  <td>
                    <IdLabel
                      id={p.id}
                      ulid={p.ulid}
                      store={entry.store}
                      {href}
                      onopen={(e) => onLinkClick(e, href)}
                    />
                  </td>
                  <td>
                    <div class="title">
                      <KindIcon kind={p.kind} />
                      <a {href} onclick={(e) => onLinkClick(e, href)}>{p.title}</a>
                    </div>
                  </td>
                  <td>
                    {#if p.progress}
                      <span class="bar" aria-hidden="true"
                        ><span class="fill" style:width="{percent(p.progress)}%"></span></span
                      >
                      <span class="frac">{p.progress.done}/{p.progress.total}</span>
                    {:else}
                      <span class="frac">—</span>
                    {/if}
                  </td>
                  <td class="due">{dueOf(p)}</td>
                  <td class="who">
                    {#if p.assignees?.[0]}
                      <Avatar name={p.assignees[0]} />
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>

      <section aria-labelledby="ov-roots">
        <div class="sec-head">
          <h2 id="ov-roots">プロジェクトの外のタスク</h2>
          <span class="sub">ルートに直接置いたタスク · rank 順 · {roots.length} 件</span>
          <a class="open-tree" href={treeHref} onclick={(e) => onLinkClick(e, treeHref)}
            >ツリーで開く</a
          >
        </div>
        {#if roots.length === 0}
          <p class="none">プロジェクトの外のタスクはありません</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th class="w-status">ステータス</th>
                <th class="w-kind">種類</th>
                <th class="w-id">ID</th>
                <th>タイトル</th>
                <th class="w-due">期限</th>
                <th class="w-who">担当者</th>
              </tr>
            </thead>
            <tbody>
              {#each roots as t (t.ulid)}
                {@const href = taskHref(t.ulid)}
                <tr>
                  <td>
                    <StatusIcon
                      status={t.status}
                      category={statusCategory(t.status)}
                      color={statusColorOf(schema, t.status)}
                    />
                  </td>
                  <td>
                    <KindIcon kind={t.kind} />
                  </td>
                  <td>
                    <IdLabel
                      id={t.id}
                      ulid={t.ulid}
                      store={entry.store}
                      {href}
                      onopen={(e) => onLinkClick(e, href)}
                    />
                  </td>
                  <td>
                    <div class="title">
                      <a {href} onclick={(e) => onLinkClick(e, href)}>{t.title}</a>
                    </div>
                  </td>
                  <td class="due">{dueOf(t)}</td>
                  <td class="who">
                    {#if t.assignees?.[0]}
                      <Avatar name={t.assignees[0]} />
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>
    {/if}
  {/if}

  <footer class="foot">
    <CliLine
      command={cliLs(baseQuery({ kind: "workspace", name: entry.name }, "cli"), true, store)}
    />
  </footer>
</div>

<style>
  .overview {
    flex: 1 1 auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-28);
    padding: var(--space-20) var(--gutter) 0;
  }
  .counts {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .total {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .band {
    display: flex;
    height: 8px;
    overflow: hidden;
    border-radius: var(--radius-pill);
    background: var(--sunken);
  }
  .seg {
    flex-basis: 0;
    min-width: 4px;
  }
  .seg[data-category="todo"] {
    background: var(--muted);
  }
  .seg[data-category="doing"] {
    background: var(--accent);
  }
  .seg[data-category="done"] {
    background: var(--success);
  }
  .seg[data-category="canceled"] {
    background: var(--muted-2);
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-16);
    margin: 0;
    padding: 0;
    list-style: none;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .legend li {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
  }
  .sec-head {
    display: flex;
    align-items: baseline;
    gap: var(--space-12);
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .sub {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .open-tree {
    margin-left: auto;
    color: var(--accent);
    font-size: var(--fs-12);
    text-decoration: none;
  }
  .open-tree:hover {
    text-decoration: underline;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th {
    padding: var(--space-6) var(--space-8);
    border-bottom: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: var(--fs-11);
    font-weight: 500;
    text-align: left;
  }
  td {
    height: var(--row-h);
    padding: 0 var(--space-8);
    border-bottom: var(--hairline) solid var(--line-soft);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-width: 0;
  }
  .title a {
    overflow: hidden;
    color: var(--ink);
    text-decoration: none;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title a:hover {
    text-decoration: underline;
  }
  .w-id {
    width: 6rem;
  }
  .w-status,
  .w-kind {
    width: 4.5rem;
  }
  .w-progress {
    width: 11rem;
  }
  .w-due {
    width: 4.5rem;
  }
  .w-who {
    width: 3.5rem;
  }
  .bar {
    display: inline-block;
    width: 5rem;
    height: 6px;
    margin-right: var(--space-8);
    overflow: hidden;
    border-radius: var(--radius-pill);
    background: var(--sunken);
    vertical-align: middle;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .frac,
  .due {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    color: var(--ink-2);
  }
  .none,
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .msg.error {
    color: var(--danger);
  }
  .foot {
    margin-top: auto;
  }
</style>
