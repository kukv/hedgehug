<script lang="ts">
  import { tick, untrack } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { CATEGORY_LABEL, type Category } from "../../core/categories";
  import { errorMessage } from "../../core/messages";
  import { schemaUsage } from "../../core/schema-usage";
  import { core } from "../context";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { onLinkClick, router, type SchemaTab } from "../router";
  import { useDisplayId, useManyStores, useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import MigrateDialog, { type MigrateTarget } from "./MigrateDialog.svelte";
  import SchemaCheck from "./SchemaCheck.svelte";
  import SettingsTemplates from "./SettingsTemplates.svelte";

  // The schema of a Store (its .hedgehug/schema.yaml), read: how many tasks use each definition, the
  // overrides of the projects, and what `schema check` finds. A name is changed or merged here (migrate);
  // everything else is edited in the YAML.
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
  // A name cannot be changed or merged in a Store that cannot be written (細部 35).
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
  const count = (n: number | undefined): string => (usage && n !== undefined ? String(n) : "—");
  const statuses = $derived(schema?.statuses ?? []);
  const kinds = $derived(schema?.kinds ?? []);
  const propEntries = $derived(Object.entries(schema?.props ?? {}));
  const transitions = $derived(Object.entries(schema?.workflow?.transitions ?? {}));
  const required = $derived(Object.entries(schema?.workflow?.required ?? {}));
  const scoped = $derived(schema?.scoped ?? []);
  const taskByUlid = $derived(new Map((tasks ?? []).map((t) => [t.ulid, t])));

  const tabItems = $derived(
    (
      [
        ["statuses", "ステータス", statuses.length],
        ["kinds", "種類", kinds.length],
        ["props", "プロパティと関連", propEntries.length],
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

  const nameList = (names: readonly string[] | null | undefined): string =>
    (names ?? []).length === 0 ? "なし" : (names ?? []).join("、");
  const parentsText = (parents: string[] | null): string =>
    parents === null
      ? "どこにでも"
      : parents.length === 0
        ? "置けない"
        : parents.map((p) => (p === "root" ? "ルート" : p)).join("、");
  const propNote = (p: { multi?: boolean; symmetric?: boolean; inverse?: string }): string =>
    [p.multi ? "複数" : "", p.symmetric ? "双方向" : "", p.inverse ? `逆方向: ${p.inverse}` : ""]
      .filter(Boolean)
      .join(" · ");

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

  // migrate: the dialog for a definition, opened from its row.
  type Ask = {
    target: MigrateTarget;
    prop?: string;
    old: string;
    mode: "rename" | "merge";
    names: string[];
  };
  let ask = $state.raw<Ask | null>(null);

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
  <p class="lead">
    {store}
    を使う全員に共有される定義です（.hedgehug/schema.yaml）。追加・削除・色や並びの変更は YAML
    を編集してください。名前の変更と併合はこの画面からできます。
  </p>

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

    {#snippet actions(target: MigrateTarget, old: string, names: string[], prop?: string)}
      {@const subject = prop ? `${prop} の ${old}` : old}
      <Button
        size="sm"
        aria-label="名前を変更…（{subject}）"
        disabled={off !== null}
        why={off ?? undefined}
        onclick={() => (ask = { target, prop, old, mode: "rename", names })}
      >
        名前を変更…
      </Button>
      {#if names.length > 1}
        <Button
          size="sm"
          aria-label="併合する…（{subject}）"
          disabled={off !== null}
          why={off ?? undefined}
          onclick={() => (ask = { target, prop, old, mode: "merge", names })}
        >
          併合する…
        </Button>
      {/if}
    {/snippet}

    <Tabs label="スキーマの種類" items={tabItems} />

    {#if tab === "statuses"}
      <section class="sec" aria-labelledby="{uid}-statuses">
        <h2 id="{uid}-statuses">ステータス</h2>
        <p class="note">
          どれも未着手・進行中・完了・中止のいずれかのカテゴリに属し、各カテゴリに既定が 1
          つあります。
        </p>
        <table aria-labelledby="{uid}-statuses">
          <thead>
            <tr>
              <th scope="col">名前</th>
              <th scope="col">表示名</th>
              <th scope="col">カテゴリ</th>
              <th scope="col">既定</th>
              <th scope="col" class="num">タスク</th>
              <th scope="col"><span class="sr">操作</span></th>
            </tr>
          </thead>
          <tbody>
            {#each statuses as s (s.name)}
              <tr>
                <th scope="row">
                  <span class="name"
                    ><StatusIcon status={s.name} category={s.category} size={14} />{s.name}</span
                  >
                </th>
                <td>{s.label ?? ""}</td>
                <td>{CATEGORY_LABEL[s.category as Category] ?? s.category}</td>
                <td>{s.default ? "既定" : ""}</td>
                <td class="num">{count(usage?.statuses[s.name])}</td>
                <td>
                  <div class="acts">
                    {@render actions(
                      "status",
                      s.name,
                      statuses.map((x) => x.name),
                    )}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {:else if tab === "templates"}
      <SettingsTemplates {store} {off} />
    {:else if tab === "kinds"}
      <section class="sec" aria-labelledby="{uid}-kinds">
        <h2 id="{uid}-kinds">種類</h2>
        <p class="note">
          役割は container（まとめる単位）と work（作業項目）の 2
          つです。置ける親に反する配置は書き込み時に拒否されます。
        </p>
        <table aria-labelledby="{uid}-kinds">
          <thead>
            <tr>
              <th scope="col">名前</th>
              <th scope="col">表示名</th>
              <th scope="col">役割</th>
              <th scope="col">置ける親</th>
              <th scope="col">既定</th>
              <th scope="col">既定のテンプレート</th>
              <th scope="col" class="num">タスク</th>
              <th scope="col"><span class="sr">操作</span></th>
            </tr>
          </thead>
          <tbody>
            {#each kinds as k (k.name)}
              <tr>
                <th scope="row">
                  <span class="name"
                    ><KindIcon kind={k.name} role={k.role} size={14} />{k.name}</span
                  >
                </th>
                <td>{k.label ?? ""}</td>
                <td>{k.role}</td>
                <td>{parentsText(k.parents)}</td>
                <td>{k.default ? "既定" : ""}</td>
                <td>{k.template ?? ""}</td>
                <td class="num">{count(usage?.kinds[k.name])}</td>
                <td>
                  <div class="acts">
                    {@render actions(
                      "kind",
                      k.name,
                      kinds.map((x) => x.name),
                    )}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {:else if tab === "props"}
      <section class="sec" aria-labelledby="{uid}-props">
        <h2 id="{uid}-props">プロパティと関連</h2>
        <p class="note">
          task_ref は同じ Store のタスクだけを参照できます。値の件数は、その値を持つタスクの数です。
        </p>
        <table aria-labelledby="{uid}-props">
          <thead>
            <tr>
              <th scope="col">名前</th>
              <th scope="col">表示名</th>
              <th scope="col">型</th>
              <th scope="col">設定</th>
              <th scope="col" class="num">タスク</th>
              <th scope="col"><span class="sr">操作</span></th>
            </tr>
          </thead>
          <tbody>
            {#each propEntries as [name, p] (name)}
              <tr>
                <th scope="row">{name}</th>
                <td>{p.label ?? ""}</td>
                <td class="mono">{p.type}</td>
                <td>{propNote(p)}</td>
                <td class="num">{count(usage?.props[name])}</td>
                <td>
                  <div class="acts">
                    {@render actions(
                      "prop",
                      name,
                      propEntries.map(([n]) => n),
                    )}
                  </div>
                </td>
              </tr>
              {#each p.values ?? [] as v (v)}
                <tr class="value">
                  <th scope="row"><span class="vname">{name} › {v}</span></th>
                  <td colspan="3"></td>
                  <td class="num">{count(usage?.values[name]?.[v])}</td>
                  <td>
                    <div class="acts">{@render actions("value", v, p.values ?? [], name)}</div>
                  </td>
                </tr>
              {/each}
            {:else}
              <tr>
                <td colspan="6" class="none">プロパティはありません</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {:else}
      <section class="sec" aria-labelledby="{uid}-workflow">
        <h2 id="{uid}-workflow">ワークフロー</h2>
        <p class="note">
          制約は、付けたステータスにだけ適用されます。付けていないステータスはどこへでも移れます。
        </p>
        <h3>遷移の制約</h3>
        {#if transitions.length === 0}
          <p class="none">制約はありません</p>
        {:else}
          <ul class="flow">
            {#each transitions as [from, to] (from)}
              <li><span class="mono">{from}</span> から移せるのは {nameList(to)} だけ</li>
            {/each}
          </ul>
        {/if}
        <h3>入るときに必須の項目</h3>
        {#if required.length === 0}
          <p class="none">必須の項目はありません</p>
        {:else}
          <ul class="flow">
            {#each required as [status, fields] (status)}
              <li><span class="mono">{status}</span> に入るには {nameList(fields)} が必要</li>
            {/each}
          </ul>
        {/if}
      </section>
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
    prop={ask.prop}
    old={ask.old}
    mode={ask.mode}
    names={ask.names}
    onclose={() => (ask = null)}
    ondone={() => void migrated()}
  />
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
  h3 {
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
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
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th,
  td {
    padding: var(--space-8) var(--space-12);
    border-bottom: 1px solid var(--line);
    text-align: left;
  }
  thead th {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  tbody th {
    font-weight: 600;
  }
  .value th {
    padding-left: var(--space-24);
    font-weight: 400;
  }
  .name {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
  }
  .num {
    text-align: right;
  }
  .acts {
    display: flex;
    gap: var(--space-6);
    justify-content: flex-end;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .none {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .flow,
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
