<script lang="ts">
  import { tick, untrack } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { schemaUsage } from "../../core/schema-usage";
  import { core } from "../context";
  import { useReadOnlySentence } from "../read-only";
  import { onLinkClick, router, type SchemaTab } from "../router";
  import { useDisplayId, useManyStores, useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import MigrateDialog, { type MigrateTarget } from "./MigrateDialog.svelte";
  import SchemaCheck from "./SchemaCheck.svelte";
  import SchemaDefDialog from "./SchemaDefDialog.svelte";
  import SchemaDeleteDialog from "./SchemaDeleteDialog.svelte";
  import SchemaKinds from "./SchemaKinds.svelte";
  import SchemaProps from "./SchemaProps.svelte";
  import SchemaRelations from "./SchemaRelations.svelte";
  import SchemaStatuses from "./SchemaStatuses.svelte";
  import SchemaWorkflow from "./SchemaWorkflow.svelte";
  import SettingsTemplates from "./SettingsTemplates.svelte";
  import type { SchemaAction } from "./schema-action";
  import { propEntries, relationEntries } from "./schema-text";

  // The schema of a Store (its .hedgehug/schema.yaml): how many tasks use each definition, the
  // overrides of the projects, and what `schema check` finds. A definition is added, edited (its name
  // too), merged and deleted from here.
  let { store, tab }: { store: string; tab: SchemaTab } = $props();

  const c = core();
  const did = useDisplayId(c.stores);
  const uid = $props.id();
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const manyStores = useManyStores(c.stores);

  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(store);
  });
  const many = $derived(manyStores.current);
  // A definition cannot be edited or merged in a Store that cannot be written (細部 35).
  const offSentence = useReadOnlySentence(c.stores, () => store);
  const off = $derived(offSentence.current);

  // The schema is what the page is made from: say so when it cannot be read, and read it again on
  // request and at resync (the cache itself only reads again what it holds).
  let heading: HTMLElement | undefined = $state();
  let schemaError = $state.raw<unknown>(null);
  let schemaTry = $state(0);
  // The button goes with the error while the schema is read, so the focus goes to the heading.
  function retry(): void {
    heading?.focus();
    schemaTry++;
  }
  $effect(() => {
    schemaTry;
    schemaError = null;
    const s = store;
    let live = true;
    c.schema.load(s).catch((e: unknown) => {
      if (live) schemaError = e;
    });
    return () => {
      live = false;
    };
  });

  let tasks = $state.raw<TaskView[] | null>(null);
  // A reload that fails keeps what was read; the failure is shown above it.
  let tasksFailure = $state<string | null>(null);
  // The latest request of each kind wins: an older answer, or one after the page is gone, is dropped.
  let tasksSeq = 0;

  async function loadTasks(s: string): Promise<void> {
    const mine = ++tasksSeq;
    try {
      const list = await c.api.listTasks({ store: s });
      if (mine !== tasksSeq) return;
      tasks = list.tasks ?? [];
      tasksFailure = null;
    } catch (e) {
      if (mine === tasksSeq) tasksFailure = errorMessage(e);
    }
  }

  let check: SchemaCheck | undefined = $state();

  // The schema cache reads the schema again at schema.changed itself, and the check follows its own
  // events; the counts follow the tasks of this Store.
  $effect(() => {
    const s = store;
    tasks = null;
    tasksFailure = null;
    ask = null; // a dialog of another Store would write to this one
    act = null;
    untrack(() => void loadTasks(s));
    const offs = [
      c.conn.on("tasks.changed", (e) => {
        if (e.store === s) void loadTasks(s);
      }),
      c.conn.on("resync", () => {
        void loadTasks(s);
        if (c.schema.get(s) === undefined) schemaTry++;
      }),
    ];
    return () => {
      for (const off of offs) off();
      tasksSeq++;
    };
  });

  const usage = $derived(schema && tasks ? schemaUsage(schema, tasks) : null);
  const propCount = $derived(propEntries(schema).length);
  const relationCount = $derived(relationEntries(schema).length);
  // blocked_by is a relation built in: it is not in the schema, and it cannot be edited.
  const blockedCount = $derived(
    tasks ? tasks.filter((t) => (t.blocked_by ?? []).length > 0).length : undefined,
  );
  const scoped = $derived(schema?.scoped ?? []);
  const taskByUlid = $derived(new Map((tasks ?? []).map((t) => [t.ulid, t])));

  const tabItems = $derived(
    (
      [
        ["statuses", "ステータス", (schema?.statuses ?? []).length],
        ["kinds", "種類", (schema?.kinds ?? []).length],
        ["props", "プロパティ", propCount],
        ["relations", "関連", relationCount + 1],
        ["workflow", "ワークフロー", undefined],
        ["templates", "テンプレート", undefined],
      ] as const
    ).map(([key, label, n]) => ({
      label,
      href: router.href({ name: "settings", page: "schema", store, tab: key }),
      current: tab === key,
      count: n,
    })),
  );

  // What a project's override changes, in a line.
  function overrideText(o: (typeof scoped)[number]): string {
    const parts: string[] = [];
    if ((o.statuses ?? []).length > 0) parts.push(`ステータス ${o.statuses?.length} 件を表示`);
    const t = Object.keys(o.workflow?.transitions ?? {}).length;
    const r = Object.keys(o.workflow?.required ?? {}).length;
    if (t > 0) parts.push(`遷移の制約 ${t} 件`);
    if (r > 0) parts.push(`必須の項目 ${r} 件`);
    return parts.join(" · ");
  }
  const projectOf = (where: string): string => where.replace(/^project:/, "");

  // migrate: the merge dialog for a definition, opened from its row.
  type Ask = { target: MigrateTarget; old: string; names: string[] };
  let ask = $state.raw<Ask | null>(null);
  // The other dialogs: add, edit, delete.
  let act = $state.raw<Exclude<SchemaAction, { kind: "migrate" }> | null>(null);

  function onaction(a: SchemaAction): void {
    if (a.kind === "migrate") ask = a;
    else act = a;
  }

  // The tasks that a refusal names by their ID, and the projects of the overrides, by what the screen read.
  const ulidById = $derived(
    new Map(
      (tasks ?? []).flatMap((t) => [
        [t.id, t.ulid] as const,
        [did.current(store, t.id), t.ulid] as const,
      ]),
    ),
  );
  const projectLabel = (ulid: string): string => {
    const p = taskByUlid.get(ulid);
    return p ? `${did.current(store, p.id)} ${p.title}` : ulid;
  };

  // The row that opened the dialog may be gone (the name changed): the focus then goes to the heading.
  async function migrated(): Promise<void> {
    // The events of the write would bring these too; reading now shows the result at once (a read that
    // an event starts after this one makes this one give way).
    void loadTasks(store);
    check?.reload();
    await tick();
    const a = document.activeElement;
    if (a === null || a === document.body) heading?.focus();
  }
</script>

<div class="page">
  <header class="head">
    <h1 bind:this={heading} tabindex="-1">スキーマ</h1>
    {#if many}
      <StoreBadge {store} tone={c.stores.tone(store)} />
    {/if}
  </header>
  <p class="lead">この Store で使う共通の定義です。ここで追加・編集・並べ替えができます。</p>

  {#if schema === undefined}
    {#if schemaError}
      <FormError message="スキーマを読み込めません · {errorMessage(schemaError)}" />
      <div><Button onclick={retry}>もう一度読み込む</Button></div>
    {:else}
      <p class="note">読み込み中…</p>
    {/if}
  {:else}
    {#if tasksFailure}
      <FormError message="タスクの件数を読み込めません · {tasksFailure}" />
    {/if}

    <Tabs label="スキーマの種類" items={tabItems} />

    {#if tab === "statuses"}
      <SchemaStatuses {store} {schema} {usage} {off} {onaction} />
    {:else if tab === "templates"}
      <SettingsTemplates {store} {off} />
    {:else if tab === "kinds"}
      <SchemaKinds {store} {schema} {usage} {off} {onaction} />
    {:else if tab === "props"}
      <SchemaProps {store} {schema} {usage} {off} {onaction} />
    {:else if tab === "relations"}
      <SchemaRelations {store} {schema} {usage} {off} {onaction} blocked={blockedCount} />
    {:else}
      <SchemaWorkflow {schema} />
    {/if}

    <section class="sec" aria-labelledby="{uid}-scoped">
      <h2 id="{uid}-scoped">プロジェクトごとの上書き</h2>
      <p class="note">定義はここで決め、プロジェクトでは設定だけを変えます。</p>
      {#if scoped.length === 0}
        <p class="none">上書きのあるプロジェクトはありません</p>
      {:else}
        <ul class="over">
          {#each scoped as o (o.where)}
            {@const ulid = projectOf(o.where)}
            {@const href = router.href({ name: "project", ulid, tab: "settings" })}
            {@const project = taskByUlid.get(ulid)}
            <li>
              <a {href} onclick={(e) => onLinkClick(e, href)}>
                {#if project}
                  <span class="mono">{did.current(store, project.id)}</span> {project.title}
                {:else}
                  プロジェクトを開く
                {/if}
              </a>
              <span class="what">{overrideText(o)}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <SchemaCheck
      {store}
      {many}
      idOf={(ulid) => {
        const id = taskByUlid.get(ulid)?.id;
        return id === undefined ? undefined : did.current(store, id);
      }}
      bind:this={check}
    />
  {/if}
</div>

{#if ask}
  <MigrateDialog
    {store}
    {many}
    target={ask.target}
    old={ask.old}
    names={ask.names}
    onclose={() => (ask = null)}
    ondone={() => void migrated()}
  />
{/if}

{#if act && schema}
  {#if act.kind === "add" || act.kind === "edit"}
    <SchemaDefDialog
      {store}
      target={act.target}
      name={act.kind === "edit" ? act.name : undefined}
      {schema}
      {usage}
      ulidOf={(id) => ulidById.get(id)}
      onclose={() => (act = null)}
      ondone={() => void migrated()}
    />
  {:else if act.kind === "delete"}
    <SchemaDeleteDialog
      {store}
      target={act.target}
      name={act.name}
      {schema}
      {projectLabel}
      onclose={() => (act = null)}
      ondone={() => void migrated()}
    />
  {/if}
{/if}

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    max-width: 920px;
  }
  .head {
    display: flex;
    gap: var(--space-10);
    align-items: center;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
  }
  h1:focus-visible {
    outline: 2px solid var(--accent);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
  }
  .lead,
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .none {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .over {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--fs-13);
  }
  .over li {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-10);
  }
  .what {
    color: var(--muted);
  }
  a {
    color: var(--accent);
  }
</style>
