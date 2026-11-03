<script lang="ts">
  import { tick, untrack } from "svelte";
  import { type Schemas, type TaskView, unwrap } from "../../core/api/client";
  import { CATEGORY_LABEL, type Category } from "../../core/categories";
  import { cliSchemaCheck } from "../../core/cli";
  import { groupFindings, type Severity, severityOf } from "../../core/doctor";
  import { errorMessage } from "../../core/messages";
  import { schemaUsage } from "../../core/schema-usage";
  import { core } from "../context";
  import Icon, { type IconName } from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import Tabs from "../ui/Tabs.svelte";
  import MigrateDialog, { type MigrateTarget } from "./MigrateDialog.svelte";

  // The schema of a Store (its .hedgehug/schema.yaml), read: how many tasks use each definition, the
  // overrides of the projects, and what `schema check` finds. A name is changed or merged here (migrate);
  // everything else is edited in the YAML.
  let { store }: { store: string } = $props();

  type Finding = Schemas["Finding"];
  type SchemaTab = "statuses" | "kinds" | "props" | "workflow";

  const c = core();
  const uid = $props.id();
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));

  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(store);
  });
  const many = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().length >= 2;
  });

  // The schema is what the page is made from: say so when it cannot be read.
  let schemaError = $state.raw<unknown>(null);
  $effect(() => {
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
  let findings = $state.raw<Finding[] | null>(null);
  // A reload that fails keeps what was read; the failure is shown above it.
  let tasksFailure = $state<string | null>(null);
  let checkFailure = $state<string | null>(null);
  // The latest request of each kind wins: an older answer, or one after the page is gone, is dropped.
  let tasksSeq = 0;
  let checkSeq = 0;

  async function loadTasks(s: string): Promise<void> {
    const mine = ++tasksSeq;
    try {
      const list = await unwrap(c.api.GET("/api/tasks", { params: { query: { store: s } } }));
      if (mine !== tasksSeq) return;
      tasks = list.tasks ?? [];
      tasksFailure = null;
    } catch (e) {
      if (mine === tasksSeq) tasksFailure = errorMessage(e);
    }
  }

  async function loadCheck(s: string): Promise<void> {
    const mine = ++checkSeq;
    try {
      const r = await unwrap(c.api.GET("/api/schema/check", { params: { query: { store: s } } }));
      if (mine !== checkSeq) return;
      findings = r.findings ?? [];
      checkFailure = null;
    } catch (e) {
      if (mine === checkSeq) checkFailure = errorMessage(e);
    }
  }

  // The schema cache reads the schema again at schema.changed itself; the counts and the check follow
  // the schema and the tasks of this Store.
  $effect(() => {
    const s = store;
    tasks = null;
    findings = null;
    tasksFailure = null;
    checkFailure = null;
    untrack(() => {
      void loadTasks(s);
      void loadCheck(s);
    });
    const all = () => {
      void loadTasks(s);
      void loadCheck(s);
    };
    const offs = [
      c.conn.on("schema.changed", (e) => {
        if (e.store === s) void loadCheck(s);
      }),
      c.conn.on("tasks.changed", (e) => {
        if (e.store === s) all();
      }),
      c.conn.on("resync", all),
    ];
    return () => {
      for (const off of offs) off();
      tasksSeq++;
      checkSeq++;
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

  let tab = $state<SchemaTab>("statuses");
  const here = $derived(router.href({ name: "settings", page: "schema", store }));
  const tabItems = $derived(
    (
      [
        ["statuses", "ステータス", statuses.length],
        ["kinds", "種類", kinds.length],
        ["props", "プロパティと関連", propEntries.length],
        ["workflow", "ワークフロー", undefined],
      ] as const
    ).map(([key, label, n]) => ({
      label,
      href: here,
      current: tab === key,
      onselect: () => (tab = key),
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
    if (t > 0) parts.push(`移動の制約 ${t} 件`);
    if (r > 0) parts.push(`必須の項目 ${r} 件`);
    return parts.join(" · ");
  }
  const projectOf = (where: string): string => where.replace(/^project:/, "");

  const groups = $derived(groupFindings(findings ?? []));
  const SEVERITY: Record<Severity, { label: string; icon: IconName }> = {
    error: { label: "エラー", icon: "forbid" },
    warning: { label: "警告", icon: "warning" },
    info: { label: "情報", icon: "status" },
  };
  const checkSummary = $derived(
    findings === null
      ? "検査しています…"
      : findings.length === 0
        ? "スキーマとタスクは合っています"
        : `${findings.length} 件の指摘があります`,
  );

  // migrate: the dialog for a definition, opened from its row.
  type Ask = {
    target: MigrateTarget;
    prop?: string;
    old: string;
    mode: "rename" | "merge";
    names: string[];
  };
  let ask = $state.raw<Ask | null>(null);
  let heading: HTMLElement | undefined = $state();

  // The row that opened the dialog may be gone (the name changed): the focus then goes to the heading.
  async function migrated(): Promise<void> {
    void loadTasks(store);
    void loadCheck(store);
    await tick();
    const a = document.activeElement;
    if (a === null || a === document.body) heading?.focus();
  }

  const lead = $derived(
    `${store} を使う全員に共有される定義です（.hedgehug/schema.yaml）。追加・削除・色や並びの変更は YAML を編集してください。名前の変更と併合はこの画面からできます。`,
  );
</script>

<div class="page">
  <header class="head">
    <h1 bind:this={heading} tabindex="-1">スキーマ</h1>
    <StoreBadge {store} tone={c.stores.tone(store)} />
  </header>
  <p class="lead">{lead}</p>

  {#if schema === undefined}
    {#if schemaError}
      <FormError message="スキーマを読めません · {errorMessage(schemaError)}" />
    {:else}
      <p class="note">読み込み中…</p>
    {/if}
  {:else}
    {#if tasksFailure}
      <FormError message="タスクの件数を読めません · {tasksFailure}" />
    {/if}

    {#snippet actions(target: MigrateTarget, old: string, names: string[], prop?: string)}
      {@const subject = prop ? `${prop} の ${old}` : old}
      <Button
        size="sm"
        aria-label="名前を変える…（{subject}）"
        onclick={() => (ask = { target, prop, old, mode: "rename", names })}
      >
        名前を変える…
      </Button>
      {#if names.length > 1}
        <Button
          size="sm"
          aria-label="併合する…（{subject}）"
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
          どれも todo・doing・done・canceled のいずれかのカテゴリに属し、各カテゴリに既定が 1
          つあります。
        </p>
        <table>
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
    {:else if tab === "kinds"}
      <section class="sec" aria-labelledby="{uid}-kinds">
        <h2 id="{uid}-kinds">種類</h2>
        <p class="note">
          役割は container（まとめる単位）と work（作業項目）の 2
          つです。置ける親に反する配置は書き込み時に拒否されます。
        </p>
        <table>
          <thead>
            <tr>
              <th scope="col">名前</th>
              <th scope="col">表示名</th>
              <th scope="col">役割</th>
              <th scope="col">置ける親</th>
              <th scope="col">既定</th>
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
        <table>
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
                  <td></td>
                  <td></td>
                  <td></td>
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
          制約を付けたステータスだけを縛ります。付けていないステータスはどこへでも移れます。
        </p>
        <h3>移動の制約</h3>
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
      <p class="note">定義はここで決め、プロジェクトでは振る舞いだけを変えます（scoped）。</p>
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
                  <span class="mono">{project.id}</span> {project.title}
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

    <section class="sec" aria-labelledby="{uid}-check">
      <h2 id="{uid}-check">スキーマの検査</h2>
      {#if checkFailure}
        <FormError message="検査の結果を読めません · {checkFailure}" />
      {/if}
      <p class="sum" aria-live="polite">{checkSummary}</p>
      {#each groups as g (g.check)}
        <div class="group">
          <h3>{g.name} <span class="n">{g.items.length}</span></h3>
          <ul class="findings">
            {#each g.items as f}
              {@const sev = severityOf(f)}
              {@const t = f.task ? taskByUlid.get(f.task) : undefined}
              <li class="finding" data-severity={sev}>
                <span class="sev"
                  ><Icon name={SEVERITY[sev].icon} size={14} />{SEVERITY[sev].label}</span
                >
                <div class="what">
                  <p class="msg">{f.msg}</p>
                  {#if f.hint}
                    <p class="hint">{f.hint}</p>
                  {/if}
                  <p class="where mono">{f.where}</p>
                  {#if f.task}
                    {@const href = router.href({ name: "task", ulid: f.task })}
                    <a class="open" {href} onclick={(e) => onLinkClick(e, href)}>
                      {t ? `${t.id} を開く` : "タスクを開く"}
                    </a>
                  {/if}
                </div>
              </li>
            {/each}
          </ul>
        </div>
      {/each}
      <div><CliLine command={cliSchemaCheck(many ? store : undefined)} /></div>
    </section>
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
  .n {
    color: var(--muted);
    font-weight: 400;
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
  .over,
  .findings {
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
  .sum {
    margin: 0;
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .finding {
    display: flex;
    gap: var(--space-10);
  }
  .sev {
    display: inline-flex;
    flex: 0 0 5rem;
    align-items: baseline;
    gap: var(--space-4);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .finding[data-severity="error"] .sev {
    color: var(--danger);
  }
  .finding[data-severity="warning"] .sev {
    color: var(--warning);
  }
  .finding[data-severity="info"] .sev {
    color: var(--muted);
  }
  .finding .what {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
    color: inherit;
  }
  .finding p {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .hint,
  .where {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .open {
    align-self: flex-start;
    font-size: var(--fs-12);
  }
</style>
