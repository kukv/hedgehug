<script lang="ts">
  import { ApiError, type Schemas, type TaskView, unwrap } from "../../core/api/client";
  import { cliMvStore, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import TaskPicker from "../detail/TaskPicker.svelte";
  import Icon from "../icons/Icon.svelte";
  import { readOnlyNow } from "../read-only";
  import { router } from "../router";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import type { MoveStoreTask } from "../workspace/dialogs-state.svelte";
  import {
    hiddenSentences,
    moveButton,
    parentSentence,
    unlinkedRows,
    violationView,
  } from "./move-plan";

  let { task, onclose }: { task: MoveStoreTask; onclose: () => void } = $props();

  const { api, cache, stores } = core();
  const did = useDisplayId(stores);
  const uid = $props.id();
  const dir = useSubscribe(stores, () => ({
    items: stores.stores(),
    workspaces: stores.workspaces(),
    why: (s: string) => readOnlyNow(stores, s),
  }));

  let to = $state("");
  // The name of the workspace picked, for a Store that has more than one; with one it is that one.
  let picked = $state("");
  let parent = $state<TaskView | null>(null);
  let scoped = $state<"carry" | "drop" | undefined>(undefined);

  // Every other Store is a place to go, a hidden one too (細部 104). One that is read-only is off.
  const targets = $derived(dir.current.items.filter((s) => s.name !== task.store));
  const spaces = $derived(dir.current.workspaces.filter((w) => w.store === to));

  const workspace = $derived(spaces.length === 1 ? (spaces[0]?.name ?? "") : picked);
  // Only a Store with more than one workspace needs the name sent (and typed in the command).
  const named = $derived(spaces.length >= 2 ? workspace : "");

  let plan = $state.raw<Schemas["MovePlanView"] | null>(null);
  let planning = $state(false);
  let busy = $state(false);
  let failure = $state<{ message: string; lines: string[] } | null>(null);
  let seq = 0;

  const fields = $derived({
    to,
    ...(named !== "" ? { workspace: named } : {}),
    ...(parent ? { parent: parent.ulid } : {}),
    ...(scoped ? { scoped } : {}),
  });

  // The parent the task has now: it may have been moved from outside while this is open.
  const taskRev = useSubscribe(cache, () => ({}));
  const from = $derived.by(() => {
    taskRev.current;
    const p = cache.task(task.ulid)?.parent ?? task.parent;
    const t = p ? cache.task(p) : undefined;
    return t ? `${did.current(t.store, t.id)} ${t.title}` : p ? "移動元の親" : null;
  });

  const cli = $derived(
    cliMvStore({
      id: did.current(task.store, task.id),
      to: to === "" ? new Placeholder("Store") : to,
      workspace:
        spaces.length >= 2 ? (named === "" ? new Placeholder("ワークスペース") : named) : undefined,
      parent: parent ? did.current(to, parent.id) : undefined,
      scoped,
    }),
  );

  function show(e: unknown) {
    const v = violationView(e);
    failure = v ? { message: v.headline, lines: v.lines } : { message: errorMessage(e), lines: [] };
  }

  // Asks for the plan of what is picked now. An answer that is not for the latest ask is dropped.
  async function refresh(): Promise<void> {
    const n = ++seq;
    plan = null;
    failure = null;
    if (to === "" || (spaces.length >= 2 && workspace === "" && parent === null)) {
      planning = false;
      return;
    }
    planning = true;
    try {
      const p = await unwrap(
        api.POST("/api/tasks/{id}/move-store/plan", {
          params: { path: { id: task.ulid } },
          body: fields,
        }),
      );
      if (n === seq) plan = p;
    } catch (e) {
      if (n === seq) show(e);
    } finally {
      if (n === seq) planning = false;
    }
  }

  function pickStore(name: string) {
    to = name;
    picked = "";
    parent = null;
    scoped = undefined; // what carrying leaves out depends on the Store
    void refresh();
  }

  function pickWorkspace(name: string) {
    picked = name;
    // The parent decides the workspace (the server takes it from the parent), so another one drops it.
    if (parent && parent.workspace.name !== name) parent = null;
    void refresh();
  }

  function pickParent(t: TaskView | null) {
    parent = t;
    if (t) picked = t.workspace.name;
    void refresh();
  }

  function pickScoped(v: "carry" | "drop") {
    scoped = v;
    void refresh();
  }

  const cut = $derived(plan?.unlinked.length ?? 0);
  const choosing = $derived(plan?.scoped !== undefined);
  const lacking = $derived(plan?.scoped?.required === true);
  const ready = $derived(plan !== null && !planning && !busy && !lacking);

  function close() {
    if (!busy) onclose();
  }

  async function submit() {
    const shown = plan;
    if (!ready || shown === null) return;
    busy = true;
    failure = null;
    try {
      const r = await unwrap(
        api.POST("/api/tasks/{id}/move-store", {
          params: { path: { id: task.ulid } },
          body: { ...fields, confirmed: shown },
        }),
      );
      // The result has no commit to note: the trees are fetched again, the lists follow tasks.changed.
      await cache.refreshTrees().catch(() => undefined);
      const t = r.tasks.find((x) => x.ulid === task.ulid) ?? r.tasks[0];
      const href = router.href({ name: "task", ulid: task.ulid });
      toast({
        message: t
          ? `${did.current(r.from, t.old_id)} を ${r.to} に移動しました（${did.current(r.to, t.new_id)}）`
          : `${did.current(task.store, task.id)} を ${r.to} に移動しました`,
        link: { label: "開く", href },
      });
      onclose();
      router.go(href);
    } catch (e) {
      busy = false;
      if (e instanceof ApiError && e.type === "move.changed") {
        await refresh();
        failure = { message: "内容が変わりました。確かめてから移動してください", lines: [] };
      } else {
        show(e);
      }
    }
  }

  const names = $derived(
    plan
      ? {
          id: `移動先の ${plan.workspace.name} で振り直します`,
          parent: parentSentence({
            from,
            parent: plan.parent ? `${plan.parent.id} ${plan.parent.title}` : null,
            workspace: plan.workspace.name,
          }),
          hidden: hiddenSentences(plan.hidden, plan.from),
        }
      : null,
  );
</script>

<Dialog open title="別の Store へ移動" width={620} {cli} cliDisabled={!ready} onclose={close}>
  <div class="form">
    <p class="what">
      <span class="mono">{did.current(task.store, task.id)}</span>
      {task.title}
      {#if plan}
        · 子孫 {plan.tasks.length - 1}
      {/if}
    </p>

    <div class="field" role="radiogroup" aria-label="移動先の Store">
      <span class="lbl">移動先</span>
      {#each targets as s (s.name)}
        {@const why = dir.current.why(s.name)}
        <!-- biome-ignore lint/a11y/noLabelWithoutControl: the badge and the remote are the text of the label -->
        <label class="choice" class:on={s.name === to} class:off={why !== null}>
          <input
            type="radio"
            name="{uid}-to"
            value={s.name}
            checked={s.name === to}
            disabled={why !== null}
            aria-describedby={why !== null ? `${uid}-${s.name}-off` : undefined}
            onchange={() => pickStore(s.name)}
          >
          <StoreBadge store={s.name} tone={stores.tone(s.name)} />
          {#if why !== null}
            <span id="{uid}-{s.name}-off" class="note">閲覧のみのため移動できません（{why}）</span>
          {:else if s.remotes?.[0]}
            <span class="note mono">{s.remotes[0]}</span>
          {/if}
        </label>
      {/each}
    </div>

    {#if to !== ""}
      <div class="row">
        {#if spaces.length >= 2}
          <label class="lbl" for="{uid}-ws">{to} のワークスペース</label>
          <select
            id="{uid}-ws"
            value={workspace}
            onchange={(e) => pickWorkspace(e.currentTarget.value)}
          >
            <option value="" disabled>選んでください</option>
            {#each spaces as w (w.ulid)}
              <option value={w.name}>{w.name}（{w.prefix}）</option>
            {/each}
          </select>
          <span class="note">{to} にはワークスペースが {spaces.length} つあります</span>
        {:else if spaces.length === 1}
          <span class="lbl">{to} のワークスペース</span>
          <span>{spaces[0]?.name}（{spaces[0]?.prefix}）</span>
        {/if}
      </div>

      <div class="row">
        <span class="lbl">親（任意）</span>
        {#if parent}
          <span><span class="mono">{did.current(to, parent.id)}</span> {parent.title}</span>
          <Button size="sm" onclick={() => pickParent(null)}>外す</Button>
        {:else}
          <div class="picker">
            <TaskPicker
              store={to}
              label="親を選ぶ"
              placeholder="親を選ぶ（ID かタイトルで検索）"
              onpick={pickParent}
            />
          </div>
        {/if}
      </div>
    {/if}

    {#if plan && names}
      <section class="part" aria-labelledby="{uid}-changes">
        <h3 id="{uid}-changes">移動先で変わること</h3>
        <dl>
          <dt>ID</dt>
          <dd>{names.id}</dd>
          <dt>親</dt>
          <dd>{names.parent}</dd>
          <dt>移動元</dt>
          <dd>転送ファイルを残し、古い ID からも辿れます</dd>
        </dl>
      </section>

      {#if cut > 0}
        <section class="part" aria-labelledby="{uid}-cut">
          <h3 id="{uid}-cut">外す参照 <span class="note">— 参照は Store の中に閉じるため</span></h3>
          <ul class="refs">
            {#each unlinkedRows(plan.unlinked) as r (r.key)}
              <li>
                <span class="kind">{r.kind}</span>
                <span class="mono">{r.target}</span>
                <span class="mono on-task">{r.id}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

      {#if choosing && plan.scoped}
        <div class="part">
          <div class="field" role="radiogroup" aria-label="プロジェクトの表示の設定">
            <span class="lbl">プロジェクトの表示の設定</span>
            <label class="plain">
              <input
                type="radio"
                name="{uid}-scoped"
                checked={scoped === "carry"}
                onchange={() => pickScoped("carry")}
              >
              持っていく
            </label>
            <label class="plain">
              <input
                type="radio"
                name="{uid}-scoped"
                checked={scoped === "drop"}
                onchange={() => pickScoped("drop")}
              >
              持っていかない
            </label>
          </div>
          {#if plan.scoped.dropped.length > 0}
            <div class="dropped">
              <span class="lbl">持っていくと外れるもの</span>
              <ul>
                {#each plan.scoped.dropped as d (d)}
                  <li>{d}</li>
                {/each}
              </ul>
            </div>
          {/if}
        </div>
      {/if}

      {#if plan.hidden.length > 0}
        <div class="warn" role="note">
          <Icon name="warning" size={16} />
          <div>
            <b>見える範囲が狭くなります</b>
            {#each names.hidden as line (line)}
              <p>{line}</p>
            {/each}
          </div>
        </div>
      {/if}
    {/if}

    {#if failure && failure.lines.length > 0}
      <div class="violations" role="alert">
        <p>{failure.message}</p>
        <ul>
          {#each failure.lines as line, i (i)}
            <li>{line}</li>
          {/each}
        </ul>
      </div>
    {:else if failure}
      <FormError message={failure.message} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={!ready} onclick={() => void submit()}>
      {busy ? "移動中…" : moveButton(cut)}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .what {
    margin: 0;
    color: var(--ink-2);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .lbl {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 400;
  }
  .choice {
    display: flex;
    gap: var(--space-10);
    align-items: center;
    min-height: 40px;
    padding: 0 var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-10);
    cursor: pointer;
  }
  .choice.on {
    border: 1.5px solid var(--accent);
    background: var(--accent-weak);
  }
  .choice.off {
    color: var(--muted);
    cursor: default;
  }
  .plain {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    align-items: center;
  }
  .picker {
    flex: 1;
    min-width: 220px;
  }
  select {
    min-height: var(--control-h);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
  }
  .part {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding-top: var(--space-12);
    border-top: 1px solid var(--line);
  }
  h3 {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  dl {
    display: grid;
    grid-template-columns: 110px minmax(0, 1fr);
    gap: var(--space-8) var(--space-12);
    margin: 0;
    line-height: 1.55;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
  }
  .refs {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .refs li {
    display: flex;
    gap: var(--space-10);
    align-items: center;
    min-height: 34px;
    padding: 0 var(--space-10);
    border-radius: var(--radius-6);
    background: var(--sunken);
  }
  .kind {
    min-width: 130px;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .on-task {
    color: var(--muted);
  }
  .dropped ul {
    margin: var(--space-4) 0 0;
    padding-left: var(--space-20);
    font-size: var(--fs-13);
  }
  .violations {
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
  .violations p {
    margin: 0;
  }
  .violations ul {
    margin: var(--space-4) 0 0;
    padding-left: var(--space-20);
  }
  .warn {
    display: flex;
    gap: var(--space-10);
    padding: var(--space-12) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--warning-weak);
    color: var(--warning);
    line-height: 1.6;
  }
  .warn p {
    margin: var(--space-6) 0 0;
  }
</style>
