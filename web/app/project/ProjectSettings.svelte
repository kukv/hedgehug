<script lang="ts">
  import { untrack } from "svelte";
  import { type TaskView, unwrap } from "../../core/api/client";
  import { cliScopeRm, cliScopeSet } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    addValues,
    draftOf,
    type Lists,
    lockedStatuses,
    putBody,
    removeValue,
    requirableFields,
    type ScopedDraft,
    setShown,
    shownStatuses,
    violationQuery,
  } from "../../core/scoped";
  import { core } from "../context";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { useListScope } from "../list/scope.svelte";
  import { onLinkClick, router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import IdLabel from "../ui/IdLabel.svelte";

  // The settings of a project: which statuses it shows and what its workflow adds to the Store's.
  // It edits a draft of the override and saves the parts that changed with one PUT.
  let { ulid }: { ulid: string } = $props();

  const c = core();
  const scope = useListScope(() => ({ kind: "project", ulid }));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const uid = $props.id();

  const store = $derived(scope.store);
  const project = $derived(scope.project);
  const schema = $derived.by(() => {
    schemaRev.current;
    return store ? c.schema.get(store) : undefined;
  });
  const manyStores = $derived.by(() => {
    storesRev.current;
    return c.stores.stores().length >= 2;
  });

  // The override the Store's schema holds for this project (the schema cache reads it again at
  // schema.changed), what the draft started from (base), and the draft.
  const held = $derived(schema?.scoped?.find((s) => s.where === `project:${ulid}`));
  const heldDraft = $derived(draftOf(held));
  const heldSig = $derived(JSON.stringify(heldDraft));
  let base = $state.raw<ScopedDraft>(draftOf(undefined));
  let draft = $state.raw<ScopedDraft>(draftOf(undefined));
  // The override base was taken from; null until the schema has been read.
  let seenSig = $state<string | null>(null);
  let violations = $state.raw<TaskView[] | null>(null);
  let busy = $state(false);
  let failure = $state<string | null>(null);
  let loadFailure = $state<string | null>(null);
  let resetOpen = $state(false);
  let resetFailure = $state<string | null>(null);
  let violationSeq = 0;

  const body = $derived(schema ? putBody(schema, base, draft) : {});
  const dirty = $derived(Object.keys(body).length > 0);
  // The Store holds something the draft did not start from (somebody else saved).
  const stale = $derived(dirty && seenSig !== null && heldSig !== seenSig);

  // A change of the override is taken into the draft unless it is being edited.
  $effect(() => {
    const sig = heldSig;
    const d = heldDraft;
    // dirty is read here, so the draft takes in the held override once an edit is undone by hand.
    const editing = dirty;
    if (!schema) return;
    untrack(() => {
      if (sig === seenSig || editing) return;
      base = d;
      draft = d;
      seenSig = sig;
    });
  });

  async function loadViolations(s: string): Promise<void> {
    const mine = ++violationSeq;
    try {
      const r = await unwrap(
        c.api.GET("/api/tasks", { params: { query: { q: violationQuery(ulid), store: s } } }),
      );
      if (mine !== violationSeq) return;
      violations = r.tasks ?? [];
      loadFailure = null;
    } catch (e) {
      if (mine === violationSeq) loadFailure = errorMessage(e);
    }
  }

  $effect(() => {
    if (!store) return;
    const s = store;
    c.schema.load(s).catch(() => undefined);
    untrack(() => void loadViolations(s));
    // The violations change with the tasks and with the schema.
    const offs = [
      c.conn.on("schema.changed", (e) => {
        if (e.store === s) void loadViolations(s);
      }),
      c.conn.on("tasks.changed", (e) => {
        if (e.store === s) void loadViolations(s);
      }),
      c.conn.on("resync", () => void loadViolations(s)),
    ];
    return () => {
      for (const off of offs) off();
      // An answer that is still on its way is not wanted any more.
      violationSeq++;
    };
  });

  const statuses = $derived(schema?.statuses ?? []);
  const shown = $derived(schema ? shownStatuses(schema, draft) : []);
  const locked = $derived(schema ? lockedStatuses(schema) : new Set<string>());
  const fields = $derived(schema ? requirableFields(schema) : []);
  const labelOf = (name: string) => statuses.find((s) => s.name === name)?.label || name;
  const storeTransitions = $derived<Lists>(
    Object.fromEntries(
      Object.entries(schema?.workflow?.transitions ?? {}).map(([k, v]) => [k, v ?? []]),
    ),
  );
  const storeRequired = $derived<Lists>(
    Object.fromEntries(
      Object.entries(schema?.workflow?.required ?? {}).map(([k, v]) => [k, v ?? []]),
    ),
  );

  // The forms that add a constraint.
  let from = $state("");
  let targets = $state<string[]>([]);
  let needFor = $state("");
  let needs = $state<string[]>([]);

  const toggle = (list: string[], v: string, on: boolean) =>
    on ? [...list, v] : list.filter((x) => x !== v);

  function addTransition() {
    const order = statuses.map((s) => s.name);
    const picked = order.filter((n) => targets.includes(n));
    draft = { ...draft, transitions: addValues(draft.transitions, from, picked) };
    from = "";
    targets = [];
  }

  function addRequired() {
    const picked = fields.filter((f) => needs.includes(f));
    draft = { ...draft, required: addValues(draft.required, needFor, picked) };
    needFor = "";
    needs = [];
  }

  function reread() {
    base = heldDraft;
    draft = heldDraft;
    seenSig = heldSig;
  }

  async function save() {
    if (busy || !dirty || !store) return;
    busy = true;
    failure = null;
    try {
      const r = await unwrap(
        c.api.PUT("/api/schema/scoped/{project}", {
          params: { path: { project: ulid }, query: { store } },
          body,
        }),
      );
      base = draftOf(r.scoped);
      draft = base;
      void c.schema.reload(store);
      toast({ message: "プロジェクトの設定を保存しました" });
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function resetAll() {
    if (busy || !store) return;
    busy = true;
    resetFailure = null;
    try {
      const r = await unwrap(
        c.api.DELETE("/api/schema/scoped/{project}", {
          params: { path: { project: ulid }, query: { store } },
        }),
      );
      base = draftOf(r.scoped);
      draft = base;
      resetOpen = false;
      void c.schema.reload(store);
      toast({ message: "プロジェクトの設定を Store の値に戻しました" });
    } catch (e) {
      resetFailure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  const cliStore = $derived(manyStores ? store : undefined);
  const setCli = $derived(project ? cliScopeSet({ id: project.id, ...body, store: cliStore }) : "");
  const rmCli = $derived(project ? cliScopeRm(project.id, cliStore) : "");
  const hasOverride = $derived(held !== undefined);
</script>

{#snippet picks(
  name: string,
  options: { value: string; label: string }[],
  chosen: string[],
  onpick: (value: string, on: boolean) => void,
)}
  <fieldset class="picks" aria-label={name}>
    {#each options as o (o.value)}
      <label class="pick">
        <input
          type="checkbox"
          checked={chosen.includes(o.value)}
          onchange={(e) => onpick(o.value, e.currentTarget.checked)}
        >
        {o.label}
      </label>
    {/each}
  </fieldset>
{/snippet}

{#snippet rows(
  name: string,
  mine: Lists,
  fromStore: Lists,
  onremove: (key: string, value: string) => void,
  arrow: boolean,
)}
  {@const items = [
    ...Object.entries(fromStore).map(([key, values]) => ({ key, values, own: false })),
    ...Object.entries(mine).map(([key, values]) => ({ key, values, own: true })),
  ]}
  {#if items.length === 0}
    <p class="msg">まだありません</p>
  {:else}
    <ul class="rows" aria-label={name}>
      {#each items as it (`${it.own ? "mine" : "store"}:${it.key}`)}
        <li class="row">
          <span class="from">{labelOf(it.key)}</span>
          <span class="arrow" aria-hidden="true">{arrow ? "→" : "に必要"}</span>
          <span class="chips">
            {#each it.values as v (v)}
              <span class="chip">
                {arrow ? labelOf(v) : v}
                {#if it.own}
                  <button
                    type="button"
                    class="drop"
                    aria-label="{arrow ? labelOf(v) : v}を外す"
                    onclick={() => onremove(it.key, v)}
                  >
                    ×
                  </button>
                {/if}
              </span>
            {/each}
          </span>
          <span class="src" class:own={it.own}
            >{it.own ? "このプロジェクトで足した" : "Store"}</span
          >
        </li>
      {/each}
    </ul>
  {/if}
{/snippet}

{#if scope.notFound}
  <Empty
    variant="notfound"
    {...scope.notFoundProps}
    onHome={() => router.go(router.href({ name: "home" }))}
  />
{:else if scope.error}
  <p class="note" role="alert">{errorMessage(scope.error)}</p>
{:else if !project || !schema || seenSig === null}
  {#if loadFailure}
    <p class="note" role="alert">{loadFailure}</p>
  {:else}
    <p class="note">読み込み中…</p>
  {/if}
{:else}
  <div class="settings">
    <p class="lead">
      このプロジェクトだけの振る舞いです。ステータスや種類の定義は
      {project.store}
      のまま変わりません。
    </p>

    {#if loadFailure}
      <FormError message={loadFailure} />
    {/if}
    {#if stale}
      <div class="stale" role="status">
        ほかで変更されました。編集中の内容はそのままです。
        <Button size="sm" onclick={reread}>取り直す</Button>
      </div>
    {/if}

    <section class="block" aria-labelledby="{uid}-statuses">
      <h2 id="{uid}-statuses">表示するステータス</h2>
      <p class="msg">
        カンバンの列と、ステータスの選択肢に出すもの · {shown.length} /
        {statuses.length}{shown.length === statuses.length ? "（Store と同じ）" : ""}
      </p>
      <fieldset class="statuses" aria-labelledby="{uid}-statuses">
        {#each statuses as s (s.name)}
          <label class="status">
            <input
              type="checkbox"
              checked={shown.includes(s.name)}
              disabled={locked.has(s.name)}
              onchange={(e) => (draft = setShown(schema, draft, s.name, e.currentTarget.checked))}
            >
            <StatusIcon status={s.name} category={s.category} />
            <span>{s.label || s.name}</span>
            <span class="name">{s.name}</span>
          </label>
        {/each}
      </fieldset>
      <p class="msg">
        カテゴリの既定は外せません（チェックを外せないものがそれです）。外したステータスのタスクはそのまま表示されます。
      </p>
      {#if shown.length !== statuses.length}
        <div>
          <Button size="sm" onclick={() => (draft = { ...draft, statuses: [] })}>
            Store と同じに戻す
          </Button>
        </div>
      {/if}
    </section>

    <section class="block" aria-labelledby="{uid}-flow">
      <h2 id="{uid}-flow">ワークフローの制約</h2>
      <p class="msg">Store の制約に足すことだけできます（緩めることはできません）</p>
      <h3>遷移の制約</h3>
      {@render rows(
        "遷移の制約",
        draft.transitions,
        storeTransitions,
        (k, v) => (draft = { ...draft, transitions: removeValue(draft.transitions, k, v) }),
        true,
      )}
      <fieldset class="add" aria-label="遷移の制約を足す">
        <select
          aria-label="遷移元のステータス"
          value={from}
          onchange={(e) => {
            from = e.currentTarget.value;
            targets = targets.filter((t) => t !== from);
          }}
        >
          <option value="">遷移元を選ぶ</option>
          {#each statuses as s (s.name)}
            <option value={s.name}>{s.label || s.name}</option>
          {/each}
        </select>
        {@render picks(
          "行ける先",
          statuses
            .filter((s) => s.name !== from)
            .map((s) => ({ value: s.name, label: s.label || s.name })),
          targets,
          (v, on) => (targets = toggle(targets, v, on)),
        )}
        <Button size="sm" disabled={from === "" || targets.length === 0} onclick={addTransition}>
          遷移の制約を足す
        </Button>
      </fieldset>

      <h3>必須の項目</h3>
      {@render rows(
        "必須の項目",
        draft.required,
        storeRequired,
        (k, v) => (draft = { ...draft, required: removeValue(draft.required, k, v) }),
        false,
      )}
      <fieldset class="add" aria-label="必須の項目を足す">
        <select
          aria-label="ステータス"
          value={needFor}
          onchange={(e) => (needFor = e.currentTarget.value)}
        >
          <option value="">ステータスを選ぶ</option>
          {#each statuses as s (s.name)}
            <option value={s.name}>{s.label || s.name}</option>
          {/each}
        </select>
        {@render picks(
          "必要な項目",
          fields.map((f) => ({ value: f, label: f })),
          needs,
          (v, on) => (needs = toggle(needs, v, on)),
        )}
        <Button size="sm" disabled={needFor === "" || needs.length === 0} onclick={addRequired}>
          必須の項目を足す
        </Button>
      </fieldset>
    </section>

    <section class="block" aria-labelledby="{uid}-bad">
      <h2 id="{uid}-bad">いまの違反</h2>
      {#if violations === null}
        <p class="msg">読み込み中…</p>
      {:else if violations.length === 0}
        <p class="msg">違反しているタスクはありません</p>
      {:else}
        <p class="msg">いまの違反 {violations.length} 件</p>
        <ul class="bad" aria-label="違反しているタスク">
          {#each violations as t (t.ulid)}
            {@const href = router.href({ name: "task", ulid: t.ulid })}
            <li>
              <IdLabel id={t.id} ulid={t.ulid} {href} onopen={(e) => onLinkClick(e, href)} />
              <span class="why">{(t.problems ?? []).map((p) => p.message).join(" / ")}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section class="block foot">
      {#if failure}
        <FormError message={failure} />
      {/if}
      <div class="row-actions">
        <Button variant="primary" disabled={busy || !dirty} onclick={() => void save()}
          >保存</Button
        >
        <Button
          variant="danger"
          disabled={busy || !hasOverride}
          onclick={() => {
            resetFailure = null;
            resetOpen = true;
          }}
        >
          すべて Store の値に戻す…
        </Button>
      </div>
      <CliLine command={setCli} disabled={!dirty} />
    </section>
  </div>

  <Dialog
    open={resetOpen}
    title="プロジェクトの設定を Store の値に戻す"
    width={560}
    cli={rmCli}
    onclose={() => (resetOpen = false)}
  >
    <div class="reset">
      <p class="msg">
        このプロジェクトのステータス・遷移の制約・必須の項目の上書きをすべて外し、{project.store}
        の値に戻します。テンプレートの上書きもあれば一緒に外れます（1 コミット）。
      </p>
      {#if resetFailure}
        <FormError message={resetFailure} />
      {/if}
    </div>
    {#snippet footer()}
      <Button disabled={busy} onclick={() => (resetOpen = false)}>やめる</Button>
      <Button variant="danger" disabled={busy} onclick={() => void resetAll()}>すべて戻す</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .note {
    margin: var(--space-16) var(--space-12);
    color: var(--muted);
  }
  .settings {
    display: flex;
    flex-direction: column;
    max-width: 760px;
    padding: var(--space-16) var(--gutter);
  }
  .lead {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .stale {
    display: flex;
    align-items: center;
    gap: var(--space-12);
    margin-top: var(--space-12);
    padding: var(--space-8) var(--space-12);
    border-radius: var(--radius-8);
    background: var(--warning-weak);
    color: var(--warning);
    font-size: var(--fs-12);
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-20) 0;
    border-bottom: 1px solid var(--line);
  }
  .block:last-child {
    border-bottom: 0;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  h3 {
    margin: var(--space-8) 0 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .statuses,
  .picks {
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8) var(--space-16);
  }
  .status,
  .pick {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    font-size: var(--fs-13);
  }
  .name {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .rows {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-8) 0;
    border-bottom: 1px solid var(--line);
    font-size: var(--fs-13);
  }
  .from {
    font-weight: 600;
  }
  .arrow {
    color: var(--muted);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: 2px var(--space-8);
    border-radius: var(--radius-pill);
    background: var(--sunken);
  }
  .drop {
    border: 0;
    background: none;
    color: var(--muted);
    font: inherit;
    cursor: pointer;
  }
  .drop:hover {
    color: var(--danger);
  }
  .src {
    margin-left: auto;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .src.own {
    color: var(--accent-ink);
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-12);
    margin: 0;
    padding: var(--space-8) 0 0;
    border: 0;
  }
  select {
    min-height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  .bad {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .bad li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-8);
    font-size: var(--fs-13);
  }
  .why {
    color: var(--muted);
  }
  .row-actions {
    display: flex;
    gap: var(--space-8);
  }
  .foot {
    border-bottom: 0;
  }
  .reset {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
</style>
