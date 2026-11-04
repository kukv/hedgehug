<script lang="ts">
  import { untrack } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliAdd, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { canParent } from "../../core/schema";
  import { core } from "../context";
  import type { CreateRequest } from "../create-state.svelte";
  import FieldInput from "../detail/FieldInput.svelte";
  import { modKey } from "../keys";
  import { readOnlyNow, useReadOnlyOf } from "../read-only";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { createGuard, resolveCreateContext, workspaceKey } from "./context";
  import { repoIdOf } from "./repos";
  import WorkspaceChoice from "./WorkspaceChoice.svelte";

  let { onclose }: { request: CreateRequest; onclose: () => void } = $props();

  const { api, cache, stores, schema } = core();
  const all = useSubscribe(stores, () => stores.workspaces());
  const roOf = useReadOnlyOf(stores);

  const ctx = untrack(() =>
    resolveCreateContext({
      selection: selection.ulid,
      route: router.route,
      task: (u) => cache.task(u),
      workspaces: stores.workspaces(),
      readOnly: (s) => readOnlyNow(stores, s),
      hidden: (s) => stores.hidden(s),
    }),
  );
  let wsKey = $state(ctx.workspace ? workspaceKey(ctx.workspace) : "");
  // The workspaces to choose from leave out a hidden Store's, except the one the dialog opened on
  // (the page of a hidden workspace is a place to create in).
  let opened = ctx.workspace ? workspaceKey(ctx.workspace) : "";
  const entries = {
    get current() {
      return all.current.filter((w) => !stores.hidden(w.store) || workspaceKey(w) === opened);
    },
  };
  const ws = $derived(entries.current.find((w) => workspaceKey(w) === wsKey));
  const showChoice = $derived(entries.current.length >= 2);
  // A workspace of a Store that cannot be written is shown but cannot be chosen (細部 35).
  const guard = $derived(createGuard(entries.current, ws, roOf.current));
  const blocked = $derived(guard.blocked);

  // Opened while the stores are still loading: decide again when they have loaded.
  $effect(() => {
    const list = all.current;
    if (wsKey !== "" || list.length === 0) return;
    const again = untrack(() =>
      resolveCreateContext({
        selection: selection.ulid,
        route: router.route,
        task: (u) => cache.task(u),
        workspaces: list,
        readOnly: (s) => readOnlyNow(stores, s),
        hidden: (s) => stores.hidden(s),
      }),
    );
    if (again.workspace) wsKey = opened = workspaceKey(again.workspace);
  });

  const sc = useSubscribe(schema, () => (ws ? schema.get(ws.store) : undefined));
  $effect(() => {
    if (ws) void schema.load(ws.store);
  });

  let title = $state("");
  let repos = $state<string[]>([]);
  let assignees = $state<string[]>([]);
  let start = $state<string | null>(null);
  let due = $state<string | null>(null);
  let busy = $state(false);
  let error = $state("");
  let titleEl = $state<HTMLInputElement>();
  $effect(() => {
    titleEl?.focus();
  });

  const projectKind = $derived.by(() => {
    const s = sc.current;
    if (!s) return "";
    if (s.kinds?.some((k) => k.name === "project")) return "project";
    return s.kinds?.find((k) => k.role === "container" && canParent(s, k.name, null))?.name ?? "";
  });
  const hasStart = $derived(!!sc.current && "start" in sc.current.props);
  const hasDue = $derived(!!sc.current && "due" in sc.current.props);
  const canSubmit = $derived(
    !busy && title.trim() !== "" && !!ws && projectKind !== "" && blocked === "",
  );

  const cli = $derived.by(() => {
    const props: Record<string, string> = {};
    if (assignees.length > 0) props.assignees = assignees.join(",");
    if (repos.length > 0) props.repos = repos.join(",");
    if (start) props.start = start;
    if (due) props.due = due;
    return cliAdd({
      title: title.trim() || new Placeholder("名前"),
      kind: projectKind || "project",
      store: ws?.store,
      workspace: showChoice ? ws?.name : undefined,
      props: Object.keys(props).length > 0 ? props : undefined,
    });
  });

  const repoHint = (text: string) => {
    const id = repoIdOf(text);
    return id ? `Enter で ${id} として追加します（SSH・HTTPS・末尾の .git を揃えます）` : null;
  };

  async function submit() {
    if (!canSubmit || !ws) return;
    busy = true;
    error = "";
    try {
      const props: Record<string, unknown> = {};
      if (start) props.start = start;
      if (due) props.due = due;
      const r = await unwrap(
        api.POST("/api/tasks", {
          params: { query: { store: ws.store } },
          body: {
            title: title.trim(),
            kind: projectKind,
            workspace: ws.name,
            ...(assignees.length > 0 ? { assignees } : {}),
            ...(repos.length > 0 ? { repos } : {}),
            ...(Object.keys(props).length > 0 ? { props } : {}),
          },
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({
        message: `${r.task.id} を作成しました · ${ws.name}`,
        link: {
          label: "開く",
          href: router.href({ name: "project", ulid: r.task.ulid, tab: "tree" }),
        },
      });
      onclose();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  function onTitleKey(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && !modKey(e)) {
      e.preventDefault();
      void submit();
    }
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
      void submit();
    }
  }}
/>

<Dialog
  open
  title="プロジェクトを作成"
  width={620}
  {cli}
  cliDisabled={title.trim() === ""}
  {onclose}
>
  <div class="form">
    <label class="field">
      <span class="lbl">名前</span>
      <input
        type="text"
        aria-label="名前"
        bind:this={titleEl}
        bind:value={title}
        onkeydown={onTitleKey}
        autocomplete="off"
      >
    </label>

    {#if showChoice}
      <div class="field">
        <WorkspaceChoice
          entries={entries.current}
          value={wsKey}
          name="create-project-ws"
          onchange={(k) => (wsKey = k)}
          isDisabled={guard.isOff}
          note={guard.note}
        />
      </div>
    {:else if ws}
      <div class="field">
        <span class="lbl">ワークスペース</span>
        <span class="fixed"
          >{ws.name}（{ws.prefix}） <StoreBadge store={ws.store} tone={ws.tone} /></span
        >
      </div>
    {/if}
    <p class="hint">
      ID はこのワークスペースの接頭辞で、作成時に振ります。子タスクも同じワークスペースに入ります
    </p>

    <div class="field">
      <span class="lbl"
        >リポジトリ <span class="hint">— なくても作れます（運用だけのプロジェクト）</span></span
      >
      <FieldInput
        editor="chips"
        label="リポジトリ"
        value={repos}
        placeholder="https://github.com/org/repo.git"
        transform={repoIdOf}
        invalidMessage="リポジトリの URL として読めません（例: https://github.com/org/repo.git）"
        hintFor={repoHint}
        onedit={(v) => (repos = v as string[])}
        oncommit={() => {}}
      />
      <span class="hint">このリポジトリの中で hh add すると、このプロジェクトに入ります</span>
    </div>

    <div class="field">
      <span class="lbl">担当者</span>
      <FieldInput
        editor="chips"
        label="担当者"
        value={assignees}
        onedit={(v) => (assignees = v as string[])}
        oncommit={() => {}}
      />
    </div>

    {#if hasStart || hasDue}
      <div class="dates">
        {#if hasStart}
          <div class="field">
            <span class="lbl">start</span>
            <FieldInput
              editor="date"
              label="start"
              value={start}
              onedit={(v) => (start = v as string | null)}
              oncommit={() => {}}
            />
          </div>
        {/if}
        {#if hasDue}
          <div class="field">
            <span class="lbl">due</span>
            <FieldInput
              editor="date"
              label="due"
              value={due}
              onedit={(v) => (due = v as string | null)}
              oncommit={() => {}}
            />
          </div>
        {/if}
      </div>
    {/if}
    {#if blocked}
      <p id="create-blocked" class="hint">{blocked}</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button onclick={onclose}>キャンセル</Button>
    <Button
      variant="primary"
      disabled={!canSubmit}
      title={blocked || undefined}
      aria-describedby={blocked ? "create-blocked" : undefined}
      onclick={() => void submit()}
      >作成</Button
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
    margin: 0;
    font-size: var(--fs-12);
    font-weight: 400;
    color: var(--muted);
  }
  .fixed {
    display: inline-flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
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
  .dates {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-12);
  }
  .error {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
