<script lang="ts">
  import { untrack } from "svelte";
  import { type ShowView, unwrap } from "../../core/api/client";
  import { cliLine } from "../../core/cli";
  import { formatDateTime } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import { copyText } from "../ui/clipboard";
  import Menu from "../ui/Menu.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { openWorkspaceMove } from "../workspace/dialogs-state.svelte";
  import { MOVE_VALUE, moveWorkspaceItem } from "../workspace/move-menu";
  import BodyEditor from "./BodyEditor.svelte";
  import Checklist from "./Checklist.svelte";
  import Comments from "./Comments.svelte";
  import { toggleChecklistLine } from "./checklist-items";
  import DeleteDialogs from "./DeleteDialogs.svelte";
  import FieldInput from "./FieldInput.svelte";
  import History from "./History.svelte";
  import { ancestorChain, type Crumb, collapseCrumbs, createLookup, crumbsSig } from "./lookup";
  import PropsPanel from "./PropsPanel.svelte";
  import Relations from "./Relations.svelte";
  import { isWatching } from "./watch";

  let { ulid }: { ulid: string } = $props();

  const { api, cache, save, schema, stores } = core();

  // Keep the task on screen watched while this route is open.
  let handle = $state<ReturnType<typeof cache.watchDetail> | null>(null);
  $effect(() => {
    const w = cache.watchDetail(ulid);
    handle = w;
    return () => w.dispose();
  });

  const shown = useSubscribe(cache, () => cache.detail(ulid));
  const loadError = useSubscribe(cache, () => handle?.error() ?? null);
  const saveState = useSubscribe(save, () => save.state(ulid));
  const bodyPending = useSubscribe(save, () => save.editing(ulid, "body"));
  const titlePending = useSubscribe(save, () => save.editing(ulid, "title"));
  const sc = useSubscribe(schema, () => {
    const d = cache.detail(ulid);
    return d ? schema.get(d.store) : undefined;
  });
  const s = $derived<ShowView | undefined>(shown.current);
  const locked = $derived(s ? s.deleted_at != null || !!s.broken : false);

  // The "…" menu of the header: the button it hangs from while it is open.
  const spaces = useSubscribe(stores, () => stores.workspaces());
  let more = $state<HTMLElement | null>(null);
  const moreItems = $derived(s ? [moveWorkspaceItem(s, spaces.current)] : []);
  function pickMore(value: string) {
    const task = s;
    more = null;
    if (task && value === MOVE_VALUE) openWorkspaceMove(task);
  }

  $effect(() => {
    const store = s?.store;
    if (store) void schema.load(store);
  });

  // Ancestors for the breadcrumb, and the blockers that are not done yet.
  let chain = $state<Crumb[]>([]);
  let blockers = $state<Crumb[]>([]);
  // The other tasks the crumbs and the blockers show change in the cache without this task changing.
  const aroundSig = useSubscribe(cache, () =>
    crumbsSig(cache, [
      ...(s?.parent ? [s.parent] : []),
      ...(s?.blocked_by ?? []).map((r) => r.ulid),
      ...chain.map((c) => c.ulid),
    ]),
  );
  const around = $derived(aroundSig.current);
  $effect(() => {
    const cur = s;
    void around;
    if (!cur) return;
    const lookup = createLookup({ api, cache, store: cur.store });
    const categories = new Map((sc.current?.statuses ?? []).map((st) => [st.name, st.category]));
    let live = true;
    void ancestorChain(cur.parent, lookup).then((c) => {
      if (live) chain = c;
    });
    void Promise.all((cur.blocked_by ?? []).map((r) => lookup(r.ulid))).then((cs) => {
      if (!live) return;
      blockers = cs.filter((c): c is Crumb => {
        if (!c) return false;
        const cat = categories.get(c.status);
        return cat !== "done" && cat !== "canceled";
      });
    });
    return () => {
      live = false;
    };
  });

  const crumbs = $derived(collapseCrumbs(chain));
  const parentCrumb = $derived(chain.at(-1)?.ulid === s?.parent ? chain.at(-1) : undefined);
  const kindLabel = $derived(
    sc.current?.kinds?.find((k) => k.name === s?.kind)?.label || s?.kind || "",
  );
  const statusLabel = (name: string) =>
    sc.current?.statuses?.find((x) => x.name === name)?.label || name;

  // Watching: the account of the store (host/login) against the watchers; what the user just did stands until the task says the same.
  const account = $derived(stores.stores().find((x) => x.name === s?.store)?.account ?? "");
  const watchingNow = $derived(isWatching(s?.watchers ?? [], account));
  let watchOverride = $state<boolean | null>(null);
  $effect(() => {
    void watchingNow;
    watchOverride = null;
  });
  const watching = $derived(watchOverride ?? watchingNow);

  const saveText = $derived.by(() => {
    const st = saveState.current;
    if (st.kind === "saving") return "保存中…";
    if (st.kind === "saved") return st.pendingPush ? "保存済み · push 待ち" : "保存済み";
    if (st.kind === "error") return st.message;
    return "";
  });

  // The body as the editor has it, for the checklist while a save is pending.
  let liveBody = $state<string | null>(null);
  $effect(() => {
    void s?.body;
    untrack(() => {
      liveBody = null;
    });
  });
  let editor = $state<{ getText(): string; setText(t: string): void } | undefined>();

  function onBodyEdit(text: string) {
    liveBody = text;
    if (s) save.edit(ulid, s.store, "body", text);
  }

  async function toggleCheck(line: number) {
    if (!s) return;
    const cur = editor?.getText() ?? s.body;
    const next = toggleChecklistLine(cur, line);
    if (next === cur) return;
    editor?.setText(next);
    liveBody = next;
    save.edit(ulid, s.store, "body", next);
    await save.flush(ulid);
  }

  async function toggleWatch() {
    if (!s) return;
    const params = { params: { path: { id: s.ulid }, query: { store: s.store } } };
    const next = !watching;
    try {
      const r = await unwrap(
        next
          ? api.PUT("/api/tasks/{id}/watchers/me", params)
          : api.DELETE("/api/tasks/{id}/watchers/me", params),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      watchOverride = next;
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  async function copyUlid() {
    if (!s) return;
    toast({
      message: (await copyText(s.ulid)) ? "ULID をコピーしました" : "コピーできませんでした",
    });
  }
</script>

{#if loadError.current}
  <p class="state" role="alert">{errorMessage(loadError.current)}</p>
{:else if !s}
  <p class="state">読み込み中…</p>
{:else}
  <div class="page">
    <header class="top">
      <nav aria-label="パンくず" class="crumbs">
        <StoreBadge store={s.store} tone={stores.tone(s.store)} />
        <span class="sep">›</span>
        <a
          href={router.href({
            name: "workspace",
            store: s.store,
            ulid: s.workspace.ulid,
            tab: "overview",
          })}
          onclick={(e) =>
            onLinkClick(
              e,
              router.href({
                name: "workspace",
                store: s.store,
                ulid: s.workspace.ulid,
                tab: "overview",
              }),
            )}
          >{s.workspace.name}</a
        >
        {#each crumbs as c, i (c ? c.ulid : `gap-${i}`)}
          <span class="sep">›</span>
          {#if c}
            <a
              href={router.href({ name: "task", ulid: c.ulid })}
              onclick={(e) => onLinkClick(e, router.href({ name: "task", ulid: c.ulid }))}
              ><span class="mono">{c.id}</span> {c.title}</a
            >
          {:else}
            <span>…</span>
          {/if}
        {/each}
        <span class="sep">›</span>
        <span class="mono current" aria-current="page">{s.id}</span>
      </nav>
      <div class="actions">
        <span class="save" role="status" aria-label="保存の状態" data-kind={saveState.current.kind}
          >{saveText}</span
        >
        {#if saveState.current.kind === "error"}
          <Button size="sm" onclick={() => void save.flush(ulid)}>やり直す</Button>
        {/if}
        <Button size="sm" onclick={toggleWatch} disabled={s.deleted_at != null}
          >{watching ? "ウォッチ中" : "ウォッチ"}</Button
        >
        {#if s.deleted_at == null && !s.broken}
          <Button
            size="sm"
            aria-label="その他の操作"
            aria-haspopup="menu"
            aria-expanded={more !== null}
            onclick={(e) => (more = e.currentTarget)}
          >
            <Icon name="more" size={14} />
          </Button>
          <DeleteDialogs show={s} variant="delete" />
          {#if more}
            <Menu
              anchor={more}
              items={moreItems}
              label="その他の操作"
              onselect={pickMore}
              onclose={() => (more = null)}
            />
          {/if}
        {/if}
      </div>
    </header>

    <div class="heading">
      <p class="eyebrow">
        <span>{kindLabel} · <span class="mono">{s.id}</span></span>
        {#if (s.former_ids ?? []).length > 0}
          <span class="muted">旧 <span class="mono">{(s.former_ids ?? []).join("、")}</span></span>
        {/if}
        <span class="muted">·</span>
        <span class="mono ulid">{s.ulid}</span>
        <button type="button" class="copy" aria-label="ULID をコピー" onclick={copyUlid}>
          コピー
        </button>
      </p>
      {#if !s.broken}
        <FieldInput
          editor="text"
          size="title"
          label="タイトル"
          value={s.title}
          nullable={false}
          pending={titlePending.current}
          readonly={locked}
          onedit={(v) => save.edit(ulid, s.store, "title", v)}
          oncommit={() => void save.flush(ulid)}
        />
      {:else}
        <h1 class="broken-title">{s.title || s.id}</h1>
      {/if}
    </div>

    {#if s.deleted_at != null}
      <div class="band deleted" role="status" aria-label="削除済み">
        <span>削除済み（{formatDateTime(s.deleted_at, new Date())}）。編集はできません。</span>
        <DeleteDialogs show={s} variant="deleted" />
      </div>
    {/if}

    {#if blockers.length > 0}
      <div class="band blocked" role="status" aria-label="ブロック">
        <span>
          {#each blockers as b, i (b.ulid)}
            {i > 0 ? "、" : ""}<span class="mono">{b.id}</span>
            {b.title}（{statusLabel(b.status)}）
          {/each}
          にブロックされています。{blockers.length === 1
            ? blockers[0]?.id
            : "すべて"}が完了か中止になれば自動で解除されます。
        </span>
      </div>
    {/if}

    {#if s.broken}
      <div class="band broken" role="alert">
        <p>このタスクのファイルを読めません: {s.broken}</p>
        <p>hh doctor で確かめてください。</p>
        <CliLine command={cliLine(["doctor"])} />
      </div>
    {:else}
      {#if (s.problems ?? []).length > 0}
        <div class="band broken" role="alert">
          <ul>
            {#each s.problems ?? [] as p (p.code + p.field)}
              <li>{p.message}</li>
            {/each}
          </ul>
          <p>hh doctor で確かめてください。</p>
          <CliLine command={cliLine(["doctor"])} />
        </div>
      {/if}

      <div class="columns">
        <article>
          <section aria-labelledby="body-h">
            <div class="sec-head">
              <h2 id="body-h">本文</h2>
              <span class="muted">Markdown</span>
            </div>
            <BodyEditor
              bind:this={editor}
              value={s.body}
              store={s.store}
              pending={bodyPending.current}
              readonly={locked}
              onedit={onBodyEdit}
              oncommit={() => void save.flush(ulid)}
            />
          </section>
          <Checklist body={liveBody ?? s.body} readonly={locked} ontoggle={toggleCheck} />
          <Relations show={s} readonly={locked} />
          <Comments show={s} readonly={locked} />
          <History show={s} readonly={locked} />
        </article>
        <aside aria-label="プロパティ">
          <PropsPanel show={s} readonly={locked} parentKind={parentCrumb?.kind} />
        </aside>
      </div>
    {/if}
  </div>
{/if}

<style>
  .page {
    display: grid;
    gap: var(--space-16);
  }
  .state {
    color: var(--muted);
  }
  .top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-12);
    flex-wrap: wrap;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: var(--space-6);
    flex-wrap: wrap;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .crumbs a {
    color: var(--ink-2);
    text-decoration: none;
  }
  .crumbs a:hover {
    text-decoration: underline;
  }
  .current {
    color: var(--ink);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  .save {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .save[data-kind="error"] {
    color: var(--danger);
  }
  .eyebrow {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    flex-wrap: wrap;
    margin: 0 0 var(--space-4);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .mono,
  .ulid {
    font-family: var(--font-mono);
  }
  .copy {
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .broken-title {
    margin: 0;
    font-size: var(--fs-19);
  }
  .band {
    display: flex;
    gap: var(--space-12);
    align-items: center;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    font-size: var(--fs-13);
  }
  .band.deleted {
    background: var(--sunken);
    color: var(--ink-2);
  }
  .band.blocked {
    background: var(--danger-weak);
    color: var(--danger);
  }
  .band.broken {
    display: block;
    background: var(--warning-weak);
    color: var(--warning);
  }
  .band p {
    margin: 0 0 var(--space-6);
  }
  .columns {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 20rem;
    gap: var(--gutter);
    align-items: start;
  }
  article {
    display: grid;
    gap: var(--space-20);
    min-width: 0;
  }
  aside {
    padding: var(--space-16);
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-12);
    background: var(--surface);
  }
  .sec-head {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .muted {
    color: var(--muted);
  }
  @media (max-width: 960px) {
    .columns {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
