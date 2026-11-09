<script lang="ts">
  import type { Schemas } from "../../core/api/client";
  import { cliSetQuery } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { bulkUpdatedLabel } from "../../core/planning";
  import { BUILTIN_PROPS, type SchemaView } from "../../core/schema";
  import { core } from "../context";
  import LabelPicker from "../detail/LabelPicker.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { runUndo } from "../undo";
  import type { BulkTarget } from "./dialogs-state.svelte";

  type Props = { target: BulkTarget; onclose: () => void };
  let { target, onclose }: Props = $props();

  type Body = Schemas.BulkInputBody;

  const c = core();
  const uid = $props.id();
  const schemaRev = useSubscribe(c.schema, () => ({}));
  // A query may find tasks of every Store: the choices are what any of them has, and the server
  // says which tasks a value does not fit.
  const names = $derived(target.store ? [target.store] : c.stores.stores().map((s) => s.name));
  $effect(() => {
    for (const s of names) void c.schema.load(s).catch(() => undefined);
  });
  const schemas = $derived.by(() => {
    schemaRev.current;
    return names.flatMap((s) => c.schema.get(s) ?? []);
  });
  const statuses = $derived([
    ...new Map(schemas.flatMap((s) => s.statuses ?? []).map((s) => [s.name, s])).values(),
  ]);
  const defs = $derived<NonNullable<SchemaView["props"]>>(
    Object.assign({}, ...schemas.map((s) => s.props ?? {})),
  );

  // The priorities and labels any of the Stores lists.
  const priorities = $derived([...new Set(schemas.flatMap((s) => s.priorities ?? []))]);
  const labelChoices = $derived([...new Set(schemas.flatMap((s) => s.labels ?? []))]);
  // Where the labels are added when there are none: the Store of the target, else the first.
  const labelStore = $derived(target.store ?? names[0] ?? "");

  const BUILTINS = [
    { value: "status", label: "ステータス" },
    { value: "sprint", label: "スプリント" },
    { value: "releases", label: "リリース" },
    { value: "assignees", label: "担当者" },
    // The built-in properties of a task: priority, start, due, estimate, points, labels.
    ...BUILTIN_PROPS.map((b) => ({ value: b.name, label: b.label })),
  ];
  const fields = $derived([
    ...BUILTINS,
    ...Object.keys(defs)
      .sort()
      .map((p) => ({ value: `props.${p}`, label: p })),
  ]);
  let field = $state("status");
  let value = $state("");
  // The labels to set, chosen from the list.
  let labels = $state<string[]>([]);

  const list = (v: string) =>
    v
      .split(",")
      .map((x) => x.trim())
      .filter(Boolean);
  // An empty value takes the field off, except a status, which every task has.
  // A number field with text that is not a number would be sent as null and clear the value.
  const isNumber = $derived(
    field === "points" || (field.startsWith("props.") && defs[field.slice(6)]?.type === "number"),
  );
  const notNumber = $derived(isNumber && value.trim() !== "" && !Number.isFinite(Number(value)));
  const ready = $derived((field !== "status" || value !== "") && !notNumber);
  const propOf = (name: string): unknown => {
    if (value === "") return null;
    return defs[name]?.type === "number" ? Number(value) : value;
  };
  const change = $derived.by<Partial<Body>>(() => {
    if (field === "status") return { status: value };
    if (field === "sprint") return { sprint: value.trim() };
    if (field === "releases") return { releases: list(value) };
    if (field === "assignees") return { assignees: list(value) };
    if (field === "labels") return { labels };
    // priority, start, due, estimate and points are fields of the body; empty clears them.
    if (field === "points") return { points: value.trim() === "" ? null : Number(value) };
    if (field === "priority" || field === "start" || field === "due" || field === "estimate") {
      return { [field]: value.trim() === "" ? null : value.trim() };
    }
    return { props: { [field.slice("props.".length)]: propOf(field.slice("props.".length)) } };
  });
  const bodyOf = (dry_run: boolean): Body => ({
    ...(target.query !== undefined ? { query: target.query } : { ulids: target.ulids }),
    ...change,
    dry_run,
  });
  const query = $derived(target.store ? { store: target.store } : {});

  // What the change would do, read again when it changes. An answer to an older change is dropped.
  let preview = $state.raw<Schemas.BulkResult | null>(null);
  let failure = $state<string | null>(null);
  $effect(() => {
    preview = null;
    if (!ready) return;
    const body = bodyOf(true);
    let live = true;
    const timer = setTimeout(() => {
      c.api
        .bulkUpdateTasks(body, query)
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
    [field.replace(/^props\./, "")]: field === "labels" ? labels.join(",") : value,
  });

  let busy = $state(false);
  async function submit() {
    if (busy || !preview || count === 0 || bad.length > 0) return;
    busy = true;
    failure = null;
    try {
      const r = await c.api.bulkUpdateTasks(bodyOf(false), query);
      const commits = r.commits ?? [];
      for (const x of commits) c.cache.noteOwnCommit(x.commit);
      const only = commits.length === 1 ? commits[0] : undefined;
      toast({
        message: bulkUpdatedLabel(r.tasks?.length ?? 0, fieldLabel, value),
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
            labels = [];
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
            {#each statuses as s (s.name)}
              <option value={s.name}>{s.label || s.name}</option>
            {/each}
          </select>
        {:else if field === "priority"}
          <select id="{uid}-v" {value} onchange={(e) => (value = e.currentTarget.value)}>
            <option value="">外す</option>
            {#each priorities as p (p)}
              <option value={p}>{p}</option>
            {/each}
          </select>
        {:else if field === "labels"}
          <LabelPicker
            store={labelStore}
            label="ラベル"
            value={labels}
            options={labelChoices}
            onedit={(v) => (labels = v)}
            oncommit={() => {}}
          />
        {:else}
          <input
            id="{uid}-v"
            type={field === "start" || field === "due" ? "date" : "text"}
            autocomplete="off"
            {value}
            placeholder={field === "releases" || field === "assignees"
              ? "カンマで区切ります。空なら外します"
              : "空なら外します"}
            oninput={(e) => (value = e.currentTarget.value)}
          >
        {/if}
      </div>
    </div>
    {#if notNumber}
      <FormError message="数を入れてください" />
    {/if}
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
