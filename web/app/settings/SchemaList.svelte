<script lang="ts" module>
  export type ListKind = "priorities" | "labels";

  const TEXT: Record<ListKind, { noun: string; note: string }> = {
    priorities: {
      noun: "優先度",
      note: "タスクの優先度は、この一覧から選びます。並びは、並べ替えとグループ化の順になります。1 つ以上は必要です。",
    },
    labels: {
      noun: "ラベル",
      note: "タスクに付けられるラベルです。タスクには、この一覧にあるものだけを付けられます。",
    },
  };
</script>

<script lang="ts">
  import "./schema-table.css";
  import { untrack } from "svelte";
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import FormError from "../ui/FormError.svelte";
  import SchemaValues, { newKey, type ValueRow } from "./SchemaValues.svelte";
  import { failureOf } from "./schema-text";

  // The priorities or the labels of the schema, edited in the same field as the values of a property:
  // add, rename, merge, delete and move. Nothing is written until the list is saved. A value that a task
  // uses cannot be deleted, only merged into another; the last priority cannot be deleted.
  type Props = {
    store: string;
    kind: ListKind;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    ondone: () => void;
  };
  let { store, kind, schema, usage, off, ondone }: Props = $props();

  const c = core();
  const uid = $props.id();
  const text = $derived(TEXT[kind]);
  const current = $derived((kind === "priorities" ? schema.priorities : schema.labels) ?? []);
  const rowsOf = (list: readonly string[]): ValueRow[] =>
    list.map((v) => ({ key: newKey(), name: v, from: [v] }));

  let rows = $state<ValueRow[]>(untrack(() => rowsOf(current)));
  let busy = $state(false);
  let err = $state.raw<unknown>(null);

  const names = $derived(rows.map((r) => r.name.trim()));
  // The list has a value that is empty, or twice: a value is told apart from another without regard to case.
  const invalid = $derived(
    names.some((n) => n === "") || new Set(names.map((n) => n.toLowerCase())).size !== names.length,
  );
  const changed = $derived(
    rows.length !== current.length ||
      rows.some(
        (r, i) =>
          r.from.length !== 1 || r.from[0] !== r.name.trim() || r.name.trim() !== current[i],
      ),
  );
  const counts = $derived(kind === "priorities" ? usage?.priorities : usage?.labels);
  // Whether a task uses any of the current values a row comes from (unknown until the tasks are read).
  const used = (from: string[]): boolean => from.some((v) => (counts?.[v] ?? 0) > 0);
  const canSave = $derived(!busy && off === null && changed && !invalid);

  // The list the server holds replaces what was typed, unless there are edits: they are the person's.
  $effect(() => {
    const list = current;
    untrack(() => {
      if (!changed) rows = rowsOf(list);
    });
  });

  async function save(): Promise<void> {
    if (!canSave) return;
    busy = true;
    err = null;
    try {
      const body = {
        values: rows.map((r) => ({
          name: r.name.trim(),
          ...(r.from.length > 0 ? { from: r.from } : {}),
        })),
      };
      if (kind === "priorities") await c.api.changePriorities(body, { store });
      else await c.api.changeLabels(body, { store });
      await c.schema.reload(store);
      const saved = c.schema.get(store);
      rows = rowsOf((kind === "priorities" ? saved?.priorities : saved?.labels) ?? []);
      toast({ message: `${text.noun}を保存しました` });
      ondone();
    } catch (e) {
      err = e;
    } finally {
      busy = false;
    }
  }

  const failure = $derived(err === null ? null : failureOf(err).text);
</script>

<section class="schema-sec" aria-labelledby="{uid}-h">
  <h2 id="{uid}-h">{text.noun}</h2>
  <p class="note">{text.note}</p>
  {#if kind === "labels" && rows.length === 0}
    <p class="note">ラベルはまだありません。「値を追加」で足します。</p>
  {/if}
  <SchemaValues
    bind:rows
    disabled={busy || off !== null}
    {used}
    keepOne={kind === "priorities" ? "優先度は 1 つ以上必要です" : undefined}
  />
  {#if invalid}
    <p class="note">値が空か、重複しています（大文字小文字は区別しません）</p>
  {/if}
  {#if off}
    <p class="note">{off}</p>
  {/if}
  {#if failure}
    <FormError message={failure} />
  {/if}
  <div>
    <Button variant="primary" disabled={!canSave} onclick={() => void save()}>保存</Button>
  </div>
</section>
