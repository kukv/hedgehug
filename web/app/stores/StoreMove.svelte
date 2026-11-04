<script lang="ts">
  import { tick, untrack } from "svelte";
  import { ApiError, type Schemas, type TaskView, unwrap } from "../../core/api/client";
  import { cliMvStore, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import TaskPicker from "../detail/TaskPicker.svelte";
  import { readOnlyNow } from "../read-only";
  import { router } from "../router";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import type { MoveStoreTask } from "../workspace/dialogs-state.svelte";
  import MovePlanSections from "./MovePlanSections.svelte";
  import { hiddenSentences, moveButton, parentSentence, violationView } from "./move-plan";

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
  // What the status region reads out while the plan is asked for and when it comes.
  let announce = $state("");
  let failBox = $state<HTMLElement | null>(null);
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
  // notice is what the dialog says meanwhile; a refusal of the plan replaces it.
  async function refresh(notice?: string): Promise<void> {
    const n = ++seq;
    plan = null;
    failure = notice ? { message: notice, lines: [] } : null;
    if (to === "" || (spaces.length >= 2 && workspace === "" && parent === null)) {
      planning = false;
      announce = "";
      return;
    }
    planning = true;
    announce = "移動の予定を取得しています…";
    try {
      const p = await unwrap(
        api.POST("/api/tasks/{id}/move-store/plan", {
          params: { path: { id: task.ulid } },
          body: fields,
        }),
      );
      if (n === seq) {
        plan = p;
        announce = "移動の予定を更新しました";
      }
    } catch (e) {
      if (n === seq) {
        announce = "";
        show(e);
      }
    } finally {
      if (n === seq) planning = false;
    }
  }

  function pickStore(name: string) {
    if (busy) return;
    to = name;
    picked = "";
    parent = null;
    scoped = undefined; // what carrying leaves out depends on the Store
    void refresh();
  }

  // The Store picked can be written no more, or is gone: it is not a choice any longer.
  // While the move runs nothing is dropped; busy is read here, so this runs again when it ends.
  $effect(() => {
    const gone = to !== "" && (!targets.some((s) => s.name === to) || dir.current.why(to) !== null);
    if (!gone || busy) return;
    untrack(() => {
      const name = to;
      to = "";
      picked = "";
      parent = null;
      scoped = undefined;
      void refresh(
        `${name} は移動先に選べなくなりました（閲覧のみになったか、なくなりました）。選び直してください`,
      );
    });
  });

  function pickWorkspace(name: string) {
    if (busy) return;
    picked = name;
    // The parent decides the workspace (the server takes it from the parent), so another one drops it.
    if (parent && parent.workspace.name !== name) parent = null;
    void refresh();
  }

  function pickParent(t: TaskView | null) {
    if (busy) return;
    parent = t;
    if (t) picked = t.workspace.name;
    void refresh();
  }

  function pickScoped(v: "carry" | "drop") {
    if (busy) return;
    scoped = v;
    void refresh();
  }

  const cut = $derived(plan?.unlinked.length ?? 0);
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
        await refresh("内容が変わりました。確かめてから移動してください");
        // Where the new plan or the notice is: it is read before the button is pressed again.
        await tick();
        failBox?.focus();
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
            parent: plan.parent
              ? `${did.current(plan.to, plan.parent.id)} ${plan.parent.title}`
              : null,
            workspace: plan.workspace.name,
          }),
          hidden: hiddenSentences(plan.hidden, plan.from),
        }
      : null,
  );
</script>

<Dialog open title="別の Store へ移動" width={620} {cli} cliDisabled={!ready} onclose={close}>
  <p class="sr" role="status">{announce}</p>
  <fieldset class="form" disabled={busy}>
    <p class="what">
      <span class="mono">{did.current(task.store, task.id)}</span>
      {task.title}
      {#if plan}
        · 子孫 {plan.tasks.length - 1}
      {/if}
    </p>

    <div class="field" role="radiogroup" aria-labelledby="{uid}-to-lbl">
      <span class="lbl" id="{uid}-to-lbl">移動先の Store</span>
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
      <MovePlanSections {plan} {names} {scoped} onscoped={pickScoped} />
    {/if}

    <div class="failbox" tabindex="-1" bind:this={failBox}>
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
  </fieldset>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={!ready} onclick={() => void submit()}>
      {busy ? "移動中…" : moveButton(cut)}
    </Button>
  {/snippet}
</Dialog>

<style>
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .failbox:focus {
    outline: none;
  }
  .form {
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
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
</style>
