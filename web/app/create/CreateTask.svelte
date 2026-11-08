<script lang="ts">
  import { untrack } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { cliAdd, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { type Template, usableTemplates } from "../../core/planning";
  import { canParent, defaultStatusIn, parentRule } from "../../core/schema";
  import { core } from "../context";
  import type { CreateRequest } from "../create-state.svelte";
  import BodyEditor from "../detail/BodyEditor.svelte";
  import FieldInput from "../detail/FieldInput.svelte";
  import PropField from "../detail/PropField.svelte";
  import { propRows, statusOptions } from "../detail/prop-rows";
  import TaskPicker from "../detail/TaskPicker.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, modKey } from "../keys";
  import { readOnlyNow, useReadOnlyOf } from "../read-only";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { choosableWorkspaces, createGuard, resolveCreateContext, workspaceKey } from "./context";
  import WorkspaceChoice from "./WorkspaceChoice.svelte";

  let { request, onclose }: { request: CreateRequest; onclose: () => void } = $props();

  const { api, cache, stores, schema } = core();
  const did = useDisplayId(stores);
  const all = useSubscribe(stores, () => stores.workspaces());
  const roOf = useReadOnlyOf(stores);

  // Where it goes is decided when the dialog opens.
  const contextOf = (workspaces: ReturnType<typeof stores.workspaces>) =>
    resolveCreateContext({
      requestParent: request.parent,
      selection: selection.ulid,
      route: router.route,
      task: (u) => cache.task(u),
      workspaces,
      readOnly: (s) => readOnlyNow(stores, s),
      hidden: (s) => stores.hidden(s),
    });
  const ctx = untrack(() => contextOf(stores.workspaces()));
  let parent = $state<TaskView | null>(ctx.parent);
  let wsKey = $state(ctx.workspace ? workspaceKey(ctx.workspace) : "");
  // Hidden Stores are left out of the choice, except the workspace the dialog opened on.
  let opened = $state(ctx.workspace ? workspaceKey(ctx.workspace) : "");
  const entries = $derived(choosableWorkspaces(all.current, (s) => stores.hidden(s), opened));
  // C pressed while the stores are still loading opens the dialog with no workspace to choose
  // from: decide again when they have loaded.
  $effect(() => {
    const list = all.current;
    if (wsKey !== "" || parent || list.length === 0) return;
    const again = untrack(() => contextOf(list));
    if (again.parent) parent = again.parent;
    if (again.workspace) wsKey = opened = workspaceKey(again.workspace);
  });
  const ws = $derived.by(() => {
    const p = parent;
    // The workspace of a parent is looked up among all workspaces on purpose: the parent may be in a
    // hidden Store (opened by its address), and it is created in its own workspace.
    if (p) return all.current.find((w) => w.store === p.store && w.ulid === p.workspace.ulid);
    return entries.find((w) => workspaceKey(w) === wsKey);
  });
  const showChoice = $derived(!parent && entries.length >= 2);
  // A workspace of a Store that cannot be written is shown but cannot be chosen (細部 35).
  const guard = $derived(createGuard(entries, ws, roOf.current));
  const blocked = $derived(guard.blocked);

  const sc = useSubscribe(schema, () => (ws ? schema.get(ws.store) : undefined));
  $effect(() => {
    if (ws) void schema.load(ws.store);
  });

  let title = $state("");
  let body = $state("");
  let kind = $state("");
  let status = $state(untrack(() => request.status ?? ""));
  // null: what the kind says; "": none.
  let pickedTemplate = $state<string | null>(null);
  let assignees = $state<string[]>([]);
  let propValues = $state<Record<string, unknown>>({});
  let busy = $state(false);
  let error = $state("");
  let titleEl = $state<HTMLInputElement>();
  let bodyEl = $state<{ setText(t: string): void } | undefined>();

  $effect(() => {
    titleEl?.focus();
  });

  const parentKind = $derived(parent?.kind ?? null);
  let templates = $state.raw<Template[]>([]);
  $effect(() => {
    const store = ws?.store;
    if (!store) return;
    let live = true;
    api
      .listTemplates({ store })
      .then((r) => {
        if (live) templates = r ?? [];
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  });
  // The project the task goes under limits the templates and may have defaults of its own.
  const scopedTemplates = $derived.by(() => {
    const p = parent;
    const project = p?.kind === "project" ? p.ulid : p?.project?.ulid;
    return sc.current?.scoped?.find((s) => s.where === `project:${project}`)?.templates;
  });
  const usable = $derived(usableTemplates(templates, scopedTemplates?.only));
  const kindDefault = $derived.by(() => {
    const name =
      scopedTemplates?.default?.[kind] ??
      sc.current?.kinds?.find((k) => k.name === kind)?.template ??
      "";
    return usable.some((t) => t.name === name) ? name : "";
  });
  const template = $derived(pickedTemplate ?? kindDefault);
  const kinds = $derived(sc.current?.kinds ?? []);
  const allowed = (name: string) => !sc.current || canParent(sc.current, name, parentKind);
  $effect(() => {
    const s = sc.current;
    if (!s) return;
    const pk = parentKind;
    const cur = untrack(() => kind);
    if (cur && s.kinds?.some((k) => k.name === cur) && canParent(s, cur, pk)) return;
    kind =
      (
        s.kinds?.find((k) => k.default && canParent(s, k.name, pk)) ??
        s.kinds?.find((k) => canParent(s, k.name, pk))
      )?.name ?? "";
  });
  $effect(() => {
    const s = sc.current;
    if (!s) return;
    if (!s.statuses?.some((x) => x.name === untrack(() => status))) {
      // A status that the request names but this workspace's schema lacks falls back to the
      // default of the category the request names (a column of the category board), else todo.
      const cat = untrack(() => request.category);
      status = (cat ? defaultStatusIn(s, cat) : "") || defaultStatusIn(s, "todo");
    }
  });

  const defaultKindName = $derived(kinds.find((k) => k.default)?.name ?? "");
  const defaultStatusName = $derived(sc.current ? defaultStatusIn(sc.current, "todo") : "");
  const rows = $derived(
    (sc.current ? propRows(sc.current, {}) : []).filter((r) => r.editor !== null),
  );
  const statusChoices = $derived(statusOptions(sc.current));
  const hold = $derived.by(() => {
    if (!parent) return "";
    const usable = kinds.filter((k) => allowed(k.name)).map((k) => k.label || k.name);
    return usable.length < kinds.length
      ? `${did.current(parent.store, parent.id)} の下に置けるのは${usable.join("・")}です`
      : "";
  });
  const canSubmit = $derived(
    !busy && title.trim() !== "" && !!ws && !!sc.current && kind !== "" && blocked === "",
  );
  const submitKey = kbdLabel("mod+enter");

  function cleanProps(): Record<string, unknown> {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(propValues)) {
      if (v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) {
        continue;
      }
      out[k] = v;
    }
    return out;
  }

  const cli = $derived.by(() => {
    const props: Record<string, string> = {};
    if (status && status !== defaultStatusName) props.status = status;
    if (assignees.length > 0) props.assignees = assignees.join(",");
    for (const [k, v] of Object.entries(cleanProps())) {
      props[k] = Array.isArray(v) ? v.join(",") : String(v);
    }
    return cliAdd({
      title: title.trim() || new Placeholder("タイトル"),
      parent: parent?.id,
      store: parent ? undefined : ws?.store,
      workspace: showChoice ? ws?.name : undefined,
      kind: kind && kind !== defaultKindName ? kind : undefined,
      template: template && template !== kindDefault ? template : undefined,
      noTemplate: template === "" && kindDefault !== "",
      body: body.trim() === "" ? undefined : body,
      props: Object.keys(props).length > 0 ? props : undefined,
    });
  });

  async function submit(more: boolean) {
    if (!canSubmit || !ws) return;
    busy = true;
    error = "";
    try {
      const props = cleanProps();
      const r = await api.createTask(
        {
          title: title.trim(),
          ...(body.trim() === "" ? {} : { body }),
          ...(parent ? { parent: parent.ulid } : { workspace: ws.name }),
          kind,
          status,
          ...(template ? { template } : kindDefault ? { no_template: true } : {}),
          ...(assignees.length > 0 ? { assignees } : {}),
          ...(Object.keys(props).length > 0 ? { props } : {}),
        },
        { store: ws.store },
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({
        message: `${did.current(ws.store, r.task.id)} を作成しました · ${ws.name}${r.children ? ` · 子タスク ${r.children} 件` : ""}`,
        link: { label: "開く", href: router.href({ name: "task", ulid: r.task.ulid }) },
      });
      if (more) {
        title = "";
        body = "";
        bodyEl?.setText("");
        titleEl?.focus();
      } else {
        onclose();
      }
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onTitleKey(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && !modKey(e)) {
      e.preventDefault();
      void submit(false);
    }
  }

  function removeParent() {
    const w = ws;
    parent = null;
    if (w) wsKey = workspaceKey(w);
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (
      e.key === "Enter" &&
      modKey(e) &&
      !e.defaultPrevented &&
      !e.isComposing &&
      e.keyCode !== 229
    ) {
      e.preventDefault();
      void submit(false);
    }
  }}
/>

<Dialog open title="タスクを作成" width={760} {cli} cliDisabled={title.trim() === ""} {onclose}>
  <div class="form">
    <div class="field">
      <span class="lbl">親</span>
      {#if parent}
        <span class="parent">
          <span class="mono">{did.current(parent.store, parent.id)}</span>
          {parent.title}
          <button type="button" aria-label="親を外す" onclick={removeParent}>×</button>
        </span>
      {:else if ws}
        <TaskPicker
          store={ws.store}
          label="親を検索"
          placeholder="親を ID かタイトルで検索（空のままならルート）"
          onpick={(t) => (parent = t)}
        />
        <span class="hint">親: なし（ワークスペースのルート）</span>
      {/if}
    </div>

    {#if showChoice}
      <div class="field">
        <WorkspaceChoice
          {entries}
          value={wsKey}
          name="create-ws"
          onchange={(k) => (wsKey = k)}
          isDisabled={guard.isOff}
          note={guard.note}
        />
        <span class="hint">全 Store にワークスペースが {entries.length} つあります</span>
      </div>
    {:else if ws}
      <div class="field">
        <span class="lbl">ワークスペース</span>
        <span class="fixed"
          >{ws.name}（{ws.prefix}） <StoreBadge store={ws.store} tone={ws.tone} /></span
        >
        <span class="hint"
          >{parent ? "親と同じワークスペースです" : `ワークスペースは ${ws.name} だけです`}</span
        >
      </div>
    {/if}

    <label class="field">
      <span class="lbl">タイトル</span>
      <input
        type="text"
        aria-label="タイトル"
        bind:this={titleEl}
        data-autofocus
        bind:value={title}
        onkeydown={onTitleKey}
        autocomplete="off"
      >
    </label>

    <div class="field">
      <span class="lbl">種類</span>
      <div class="kinds" role="radiogroup" aria-label="種類">
        {#each kinds as k (k.name)}
          {@const usable = allowed(k.name)}
          <label
            class:on={kind === k.name}
            class:off={!usable}
            title={usable || !sc.current ? undefined : parentRule(sc.current, k.name)}
          >
            <input
              type="radio"
              name="create-kind"
              value={k.name}
              checked={kind === k.name}
              disabled={!usable}
              onchange={() => (kind = k.name)}
            >
            <KindIcon kind={k.name} role={k.role} icon={k.icon} />
            {k.label || k.name}
          </label>
        {/each}
      </div>
      {#if hold}
        <span class="hint">{hold}</span>
      {/if}
    </div>

    {#if usable.length > 0}
      <div class="field">
        <label class="lbl" for="create-template">テンプレート</label>
        <select
          id="create-template"
          value={template}
          onchange={(e) => (pickedTemplate = e.currentTarget.value)}
        >
          <option value="">なし</option>
          {#each usable as t (t.name)}
            <option value={t.name}>
              {t.name}{t.children > 0 ? `（子タスク ${t.children}）` : ""}
            </option>
          {/each}
        </select>
      </div>
    {/if}

    <div class="field">
      <span class="lbl">本文 <span class="hint">Markdown · @ で人、# でタスク</span></span>
      {#if ws}
        <BodyEditor
          bind:this={bodyEl}
          value={body}
          store={ws.store}
          label="本文"
          minHeight="6em"
          onedit={(t) => (body = t)}
          onsubmit={() => void submit(false)}
        />
      {/if}
    </div>

    <div class="props">
      <div class="prow">
        <span class="lbl">ステータス</span>
        <FieldInput
          editor="select"
          label="ステータス"
          value={status}
          options={statusChoices}
          nullable={false}
          onedit={(v) => (status = String(v))}
          oncommit={() => {}}
        />
      </div>
      <div class="prow">
        <span class="lbl">担当者</span>
        <FieldInput
          editor="chips"
          label="担当者"
          value={assignees}
          onedit={(v) => (assignees = v as string[])}
          oncommit={() => {}}
        />
      </div>
      {#each rows as r (r.name)}
        <div class="prow">
          <span class="lbl">{r.label}</span>
          <PropField
            row={r}
            value={propValues[r.name] ?? null}
            onedit={(v) => (propValues[r.name] = v)}
            oncommit={() => {}}
          />
        </div>
      {/each}
    </div>

    {#if ws}
      <p class="dest">作成先 <strong>{ws.name}（{ws.prefix}）</strong> · ID は作成時に振ります</p>
    {/if}
    {#if blocked}
      <p id="create-blocked" class="hint">{blocked}</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button
      disabled={!canSubmit}
      title={blocked || undefined}
      aria-describedby={blocked ? "create-blocked" : undefined}
      onclick={() => void submit(true)}
      >続けて作成</Button
    >
    <Button onclick={onclose}>キャンセル</Button>
    <Button
      variant="primary"
      disabled={!canSubmit}
      title={blocked || undefined}
      aria-describedby={blocked ? "create-blocked" : undefined}
      onclick={() => void submit(false)}
      >作成 {submitKey}</Button
    >
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-12);
  }
  .field {
    display: grid;
    gap: var(--space-4);
  }
  .lbl {
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .hint {
    font-size: var(--fs-12);
    font-weight: 400;
    color: var(--muted);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .parent,
  .fixed {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
  }
  .parent button {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
  }
  input[type="text"] {
    width: 100%;
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font: inherit;
    font-size: var(--fs-13);
    color: var(--ink);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
  .kinds {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
  }
  .kinds label {
    position: relative;
    display: inline-flex;
    gap: var(--space-4);
    align-items: center;
    padding: var(--space-4) var(--space-10);
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-13);
    cursor: pointer;
  }
  .kinds label:focus-within {
    outline: 2px solid var(--accent);
  }
  .kinds label.on {
    background: var(--ink);
    color: var(--surface);
  }
  .kinds label.off {
    background: transparent;
    box-shadow: inset 0 0 0 1px var(--line);
    color: var(--muted-2);
    text-decoration: line-through;
    cursor: not-allowed;
  }
  .kinds input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .props {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-6) var(--space-16);
  }
  .prow {
    display: grid;
    grid-template-columns: 7em minmax(0, 1fr);
    align-items: center;
    gap: var(--space-8);
  }
  .dest {
    margin: 0;
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  .error {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
