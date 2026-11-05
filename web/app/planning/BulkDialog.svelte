<script lang="ts">
  import { type Schemas, unwrap } from "../../core/api/client";
  import { cliSetQuery } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { runUndo } from "../undo";
  import type { BulkTarget } from "./dialogs-state.svelte";

  type Props = { target: BulkTarget; onclose: () => void };
  let { target, onclose }: Props = $props();

  type Body = Schemas["BulkInputBody"];

  const c = core();
  const uid = $props.id();
  const schemaRev = useSubscribe(c.schema, () => ({}));
  const store = $derived(target.store ?? c.stores.stores()[0]?.name ?? "");
  $effect(() => {
    if (store) void c.schema.load(store).catch(() => undefined);
  });
  const schema = $derived.by(() => {
    schemaRev.current;
    return c.schema.get(store);
  });

  const BUILTINS = [
    { value: "status", label: "ステータス" },
    { value: "sprint", label: "スプリント" },
    { value: "releases", label: "リリース" },
    { value: "assignees", label: "担当者" },
    { value: "labels", label: "ラベル" },
  ];
  const fields = $derived([
    ...BUILTINS,
    ...Object.keys(schema?.props ?? {})
      .sort()
      .map((p) => ({ value: `props.${p}`, label: p })),
  ]);
  let field = $state("status");
  let value = $state("");

  const list = (v: string) =>
    v
      .split(",")
      .map((x) => x.trim())
      .filter(Boolean);
  // An empty value takes the field off, except a status, which every task has.
  const ready = $derived(field !== "status" || value !== "");
  const propOf = (name: string): unknown => {
    const def = schema?.props?.[name];
    if (value === "") return null;
    return def?.type === "number" ? Number(value) : value;
  };
  const change = $derived.by<Partial<Body>>(() => {
    if (field === "status") return { status: value };
    if (field === "sprint") return { sprint: value.trim() };
    if (field === "releases") return { releases: list(value) };
    if (field === "assignees") return { assignees: list(value) };
    if (field === "labels") return { labels: list(value) };
    return { props: { [field.slice("props.".length)]: propOf(field.slice("props.".length)) } };
  });
  const bodyOf = (dry_run: boolean): Body => ({
    ...(target.query !== undefined ? { query: target.query } : { ulids: target.ulids }),
    ...change,
    dry_run,
  });
  const params = $derived({ query: target.store ? { store: target.store } : {} });

  // What the change would do, read again when it changes. An answer to an older change is dropped.
  let preview = $state.raw<Schemas["BulkResult"] | null>(null);
  let failure = $state<string | null>(null);
  $effect(() => {
    preview = null;
    if (!ready) return;
    const body = bodyOf(true);
    let live = true;
    const timer = setTimeout(() => {
      unwrap(c.api.POST("/api/tasks/bulk", { params, body }))
        .then((r) => {
          if (!live) return;
          preview = r;
          failure = null;
        })
        .catch((e: unknown) => {
          if (live) failure = errorMessage(e);
        });
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  });
  const count = $derived(preview?.tasks?.length ?? 0);
  const bad = $derived(preview?.violations ?? []);
  const fieldLabel = $derived(fields.find((f) => f.value === field)?.label ?? field);
  const assigns = $derived<Record<string, string>>({
    [field.replace(/^props\./, "")]: value,
  });

  let busy = $state(false);
  async function submit() {
    if (busy || !preview || count === 0 || bad.length > 0) return;
    busy = true;
    failure = null;
    try {
      const r = await unwrap(c.api.POST("/api/tasks/bulk", { params, body: bodyOf(false) }));
      const commits = r.commits ?? [];
      for (const x of commits) c.cache.noteOwnCommit(x.commit);
      const only = commits.length === 1 ? commits[0] : undefined;
      toast({
        message: `${r.tasks?.length ?? 0} 件の${fieldLabel}を${value === "" ? "外し" : `${value} に`}しました`,
        undo: only ? () => runUndo(c, only.store) : undefined,
      });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open
  title={target.query !== undefined ? "一致するタスクをまとめて更新" : "選んだタスクをまとめて更新"}
  width={620}
  cli={target.query !== undefined ? cliSetQuery(target.query, assigns, target.store) : undefined}
  {onclose}
>
  <div class="form">
    <div class="row">
      <div class="field">
        <label for="{uid}-f">項目</label>
        <select
          id="{uid}-f"
          data-autofocus
          value={field}
          onchange={(e) => {
            field = e.currentTarget.value;
            value = "";
          }}
        >
          {#each fields as f (f.value)}
            <option value={f.value}>{f.label}</option>
          {/each}
        </select>
      </div>
      <div class="field grow">
        <label for="{uid}-v">値</label>
        {#if field === "status"}
          <select id="{uid}-v" {value} onchange={(e) => (value = e.currentTarget.value)}>
            <option value="">選んでください</option>
            {#each schema?.statuses ?? [] as s (s.name)}
              <option value={s.name}>{s.label || s.name}</option>
            {/each}
          </select>
        {:else}
          <input
            id="{uid}-v"
            type="text"
            autocomplete="off"
            {value}
            placeholder={field === "releases" || field === "assignees" || field === "labels"
              ? "カンマで区切ります。空なら外します"
              : "空なら外します"}
            oninput={(e) => (value = e.currentTarget.value)}
          >
        {/if}
      </div>
    </div>
    {#if preview}
      <p class="count">{count} 件が対象です{bad.length > 0 ? ` · 違反 ${bad.length} 件` : ""}</p>
      {#if bad.length > 0}
        <ul class="bad" aria-label="違反">
          {#each bad as v (v.task.ulid + v.field)}
            <li>{v.task.id}: {v.message}</li>
          {/each}
        </ul>
      {/if}
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button
      variant="primary"
      disabled={busy || !preview || count === 0 || bad.length > 0}
      onclick={() => void submit()}
    >
      {count}
      件を更新
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-12);
  }
  .field {
    display: flex;
    flex: 0 1 180px;
    flex-direction: column;
    gap: var(--space-6);
  }
  .field.grow {
    flex: 1 1 240px;
  }
  label {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  select,
  input {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    font-size: var(--fs-14);
  }
  .count {
    margin: 0;
    font-weight: 600;
  }
  .bad {
    margin: 0;
    padding-left: var(--space-16);
    color: var(--danger);
  }
</style>
