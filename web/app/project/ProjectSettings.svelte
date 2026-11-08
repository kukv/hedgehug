<script lang="ts">
  import { untrack } from "svelte";
  import { cliScopeRm, cliScopeSet } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import {
    draftOf,
    type Lists,
    lockedStatuses,
    putBody,
    requirableFields,
    type ScopedDraft,
    setShown,
    shownStatuses,
  } from "../../core/scoped";
  import { core } from "../context";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { useListScope } from "../list/scope.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { router } from "../router";
  import Empty from "../states/Empty.svelte";
  import { useManyStores, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import ConstraintEditor from "./ConstraintEditor.svelte";
  import ProjectAutomations from "./ProjectAutomations.svelte";
  import ProjectTemplates from "./ProjectTemplates.svelte";
  import ProjectViolations from "./ProjectViolations.svelte";

  // The settings of a project: which statuses it shows and what its workflow adds to the Store's.
  // It edits a draft of the override and saves the parts that changed with one PUT.
  let { ulid }: { ulid: string } = $props();

  const c = core();
  const scope = useListScope(() => ({ kind: "project", ulid }));
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const manyStores = useManyStores(c.stores);
  const uid = $props.id();

  const store = $derived(scope.store);
  // A Store that cannot be written is read, not edited (細部 35).
  const offSentence = useReadOnlySentence(c.stores, () => store);
  const off = $derived(offSentence.current);
  const project = $derived(scope.project);
  const schema = $derived.by(() => {
    schemaRev.current;
    return store ? c.schema.get(store) : undefined;
  });

  // The schema of the Store is what the screen is made from: say so when it cannot be read.
  // It is read again on request and at resync (the cache itself only reads again what it holds).
  let schemaError = $state.raw<unknown>(null);
  let schemaTry = $state(0);
  // The button goes with the error while the schema is read, so the focus goes to the heading of the
  // page (the one of ProjectHeader).
  function retry(): void {
    document.querySelector<HTMLElement>("h1")?.focus();
    schemaTry++;
  }
  $effect(() => {
    schemaTry;
    schemaError = null;
    const s = store;
    if (!s) return;
    c.schema.load(s).catch((e: unknown) => {
      if (s === store) schemaError = e;
    });
  });
  $effect(() =>
    c.conn.on("resync", () => {
      if (store && c.schema.get(store) === undefined) schemaTry++;
    }),
  );

  // The override the Store's schema holds for this project (the schema cache reads it again at
  // schema.changed), what the draft started from (base), and the draft.
  const held = $derived(schema?.scoped?.find((s) => s.where === `project:${ulid}`));
  const heldDraft = $derived(draftOf(held));
  const heldSig = $derived(JSON.stringify(heldDraft));
  let base = $state.raw<ScopedDraft>(draftOf(undefined));
  let draft = $state.raw<ScopedDraft>(draftOf(undefined));
  // The override base was taken from; null until the schema has been read.
  let seenSig = $state<string | null>(null);
  // A request is on its way: the draft is not to change under it.
  let busy = $state(false);
  // What the controls follow: a request on its way, or a Store that cannot be written.
  const frozen = $derived(busy || off !== null);
  let failure = $state<string | null>(null);
  let resetOpen = $state(false);
  let resetFailure = $state<string | null>(null);

  const body = $derived(schema ? putBody(schema, base, draft) : {});
  const dirty = $derived(Object.keys(body).length > 0);
  // The Store holds something the draft did not start from (somebody else saved). Not while our
  // own write is being taken in.
  const stale = $derived(!busy && dirty && seenSig !== null && heldSig !== seenSig);

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

  const statuses = $derived(schema?.statuses ?? []);
  const shown = $derived(schema ? shownStatuses(schema, draft) : []);
  const locked = $derived(schema ? lockedStatuses(schema) : new Set<string>());
  const fields = $derived(schema ? requirableFields(schema) : []);
  const listsOf = (m: Record<string, string[] | null> | undefined): Lists =>
    Object.fromEntries(Object.entries(m ?? {}).map(([k, v]) => [k, v ?? []]));
  const storeTransitions = $derived(listsOf(schema?.workflow?.transitions));
  const storeRequired = $derived(listsOf(schema?.workflow?.required));

  function reread() {
    base = heldDraft;
    draft = heldDraft;
    seenSig = heldSig;
  }

  // takeIn makes the response of a write the new start of the draft. The schema is read again
  // first, so that what the Store holds is what the draft started from.
  async function takeIn(scoped: Parameters<typeof draftOf>[0], s: string) {
    await c.schema.reload(s);
    base = draftOf(scoped);
    draft = base;
    seenSig = heldSig;
  }

  async function save() {
    if (busy || off !== null || !dirty || !store) return;
    busy = true;
    failure = null;
    try {
      const r = await c.api.putScoped(ulid, body, { store });
      await takeIn(r.scoped, store);
      toast({ message: "プロジェクトの設定を保存しました" });
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function resetAll() {
    if (busy || off !== null || !store) return;
    busy = true;
    resetFailure = null;
    try {
      const r = await c.api.deleteScoped(ulid, { store });
      await takeIn(r.scoped, store);
      resetOpen = false;
      toast({ message: "プロジェクトの設定を Store の値に戻しました" });
    } catch (e) {
      resetFailure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  const cliStore = $derived(manyStores.current ? store : undefined);
  const setCli = $derived(
    project
      ? cliScopeSet({
          id: project.id,
          ...body,
          templateDefaults: body.template_defaults,
          store: cliStore,
        })
      : "",
  );
  const rmCli = $derived(project ? cliScopeRm(project.id, cliStore) : "");
  const hasOverride = $derived(held !== undefined);
</script>

{#if scope.notFound}
  <Empty
    variant="notfound"
    {...scope.notFoundProps}
    onHome={() => router.go(router.href({ name: "home" }))}
  />
{:else if scope.error}
  <p class="note" role="alert">{errorMessage(scope.error)}</p>
{:else if !project || !store || !schema || seenSig === null}
  {#if schemaError}
    <FormError message="スキーマを読み込めません · {errorMessage(schemaError)}" />
    <div><Button onclick={retry}>もう一度読み込む</Button></div>
  {:else}
    <p class="note">読み込み中…</p>
  {/if}
{:else}
  <div class="settings">
    <p class="lead">
      このプロジェクトだけの設定です。ステータスや種類の定義は
      {project.store}
      のまま変わりません。
    </p>

    {#if stale}
      <div class="stale" role="status">
        ほかで変更されました。編集中の内容はそのままです。
        <Button size="sm" onclick={reread}>もう一度読み込む</Button>
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
              disabled={frozen || locked.has(s.name)}
              aria-describedby={locked.has(s.name) ? `${uid}-locked` : undefined}
              onchange={(e) => (draft = setShown(schema, draft, s.name, e.currentTarget.checked))}
            >
            <StatusIcon status={s.name} category={s.category} color={s.color} />
            <span>{s.label || s.name}</span>
            <span class="name">{s.name}</span>
          </label>
        {/each}
      </fieldset>
      <p class="msg" id="{uid}-locked">
        カテゴリの既定は外せません（チェックを外せない項目です）。外したステータスのタスクはそのまま表示されます。
      </p>
      {#if shown.length !== statuses.length}
        <div>
          <Button
            size="sm"
            disabled={frozen}
            why={off ?? undefined}
            onclick={() => (draft = { ...draft, statuses: [] })}
          >
            Store と同じに戻す
          </Button>
        </div>
      {/if}
    </section>

    <section class="block" aria-labelledby="{uid}-flow">
      <h2 id="{uid}-flow">ワークフローの制約</h2>
      <p class="msg">Store の制約に追加することだけできます（緩めることはできません）</p>
      <ConstraintEditor
        mode="transition"
        {statuses}
        {fields}
        mine={draft.transitions}
        fromStore={storeTransitions}
        busy={frozen}
        onchange={(transitions) => (draft = { ...draft, transitions })}
      />
      <ConstraintEditor
        mode="required"
        {statuses}
        {fields}
        mine={draft.required}
        fromStore={storeRequired}
        busy={frozen}
        onchange={(required) => (draft = { ...draft, required })}
      />
    </section>

    <ProjectTemplates {store} {schema} {draft} busy={frozen} onchange={(d) => (draft = d)} />

    <ProjectAutomations {store} {schema} {project} {off} />

    <ProjectViolations {ulid} {store} />

    <section class="block foot">
      {#if failure}
        <FormError message={failure} />
      {/if}
      <div class="row-actions">
        <Button
          variant="primary"
          disabled={frozen || !dirty}
          why={off ?? undefined}
          onclick={() => void save()}
          >保存</Button
        >
        <Button
          variant="danger"
          disabled={frozen || !hasOverride}
          why={off ?? undefined}
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
    onclose={() => {
      // Esc and the close button do nothing while the request is on its way.
      if (!busy) resetOpen = false;
    }}
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
      <Button disabled={busy} onclick={() => (resetOpen = false)}>キャンセル</Button>
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
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .statuses {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8) var(--space-16);
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  .status {
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
