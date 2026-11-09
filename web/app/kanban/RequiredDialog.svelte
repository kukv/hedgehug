<script lang="ts">
  import type { Schemas, TaskView } from "../../core/api/client";
  import { cliSet } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { isBuiltinProp, type SchemaView } from "../../core/schema";
  import { core } from "../context";
  import FieldInput from "../detail/FieldInput.svelte";
  import LabelPicker from "../detail/LabelPicker.svelte";
  import PropField from "../detail/PropField.svelte";
  import { builtinRows, propRows } from "../detail/prop-rows";
  import { onLinkClick, router } from "../router";
  import { useDisplayId } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import { requiredLabel } from "./drop";

  type Props = {
    task: TaskView;
    status: string; // the status the card is dropped on
    fields: string[]; // the required fields the task lacks and this dialog takes in (requiredGap.fill)
    elsewhere: string[]; // the ones only the task detail can fill (requiredGap.elsewhere)
    schema: SchemaView;
    // Sends the status and the fields in one PATCH. A failure is thrown and shown here.
    onsubmit: (body: Schemas.PatchInputBody) => Promise<void>;
    onclose: () => void;
  };
  const did = useDisplayId(core().stores);
  let { task, status, fields, elsewhere, schema, onsubmit, onclose }: Props = $props();

  const statusLabel = $derived(schema.statuses?.find((s) => s.name === status)?.label || status);
  // The built-in properties and the properties of the schema.
  const rows = $derived([...builtinRows(schema, {}), ...propRows(schema, {})]);
  const rowOf = (name: string) => rows.find((r) => r.name === name);
  const labelOf = (name: string): string => requiredLabel(schema, name);
  const detailHref = $derived(router.href({ name: "task", ulid: task.ulid }));

  let assignees = $state<string[]>([]);
  let labels = $state<string[]>([]);
  let values = $state<Record<string, unknown>>({});
  let busy = $state(false);
  let error = $state("");

  const empty = (v: unknown) =>
    v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0);
  const currentOf = (name: string): unknown =>
    name === "assignees" ? assignees : name === "labels" ? labels : values[name];
  const complete = $derived(fields.every((f) => !empty(currentOf(f))));

  const title = $derived(
    `${did.current(task.store, task.id)} を${statusLabel}にするには ${[...fields, ...elsewhere].map(labelOf).join("・")} が必要です`,
  );
  const cli = $derived(
    cliSet(did.current(task.store, task.id), {
      status,
      ...Object.fromEntries(
        fields
          .filter((f) => !empty(currentOf(f)))
          .map((f) => {
            const v = currentOf(f);
            return [f, Array.isArray(v) ? v.join(",") : String(v)];
          }),
      ),
    }),
  );

  async function submit() {
    if (!complete || elsewhere.length > 0 || busy) return;
    busy = true;
    error = "";
    const body: Schemas.PatchInputBody = { status };
    if (fields.includes("assignees")) body.assignees = assignees;
    if (fields.includes("labels")) body.labels = labels;
    // A built-in property is a field of the body; a property of the schema goes in props.
    const others = fields.filter((f) => f !== "assignees" && f !== "labels");
    const sent = Object.fromEntries(
      others.filter((f) => !isBuiltinProp(f)).map((f) => [f, values[f]]),
    );
    Object.assign(
      body,
      Object.fromEntries(others.filter(isBuiltinProp).map((f) => [f, values[f]])),
    );
    if (Object.keys(sent).length > 0) body.props = sent;
    try {
      await onsubmit(body);
      onclose();
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open {title} width={560} {cli} cliDisabled={!complete || elsewhere.length > 0} {onclose}>
  <div class="form">
    {#each fields as name (name)}
      <div class="prow">
        <span class="lbl">{labelOf(name)}</span>
        {#if name === "assignees"}
          <FieldInput
            editor="chips"
            label="担当者"
            value={assignees}
            onedit={(v) => (assignees = v as string[])}
            oncommit={() => {}}
          />
        {:else if name === "labels"}
          <LabelPicker
            store={task.store}
            label="ラベル"
            value={labels}
            options={schema.labels ?? []}
            onedit={(v) => (labels = v)}
            oncommit={() => {}}
          />
        {:else}
          {@const row = rowOf(name)}
          {#if row}
            <PropField
              {row}
              value={values[name] ?? null}
              onedit={(v) => (values[name] = v)}
              oncommit={() => {}}
            />
          {/if}
        {/if}
      </div>
    {/each}
    {#each elsewhere as name (name)}
      <div class="prow">
        <span class="lbl">{labelOf(name)}</span>
        <a
          class="detail"
          href={detailHref}
          onclick={(e) => {
            onLinkClick(e, detailHref);
            onclose();
          }}
          >詳細で入力</a
        >
      </div>
    {/each}
    {#if elsewhere.length > 0}
      <p class="note">
        {fields.length > 0
          ? "ここで入力できない項目"
          : "この項目"}はタスクの詳細で入力してください。入力するまで{statusLabel}にはできません。
      </p>
    {:else}
      <p class="note">ステータスと入力した項目は、1 回の保存（1 コミット）で変わります。</p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>

  {#snippet footer()}
    <Button onclick={onclose}>キャンセル</Button>
    <Button
      variant="primary"
      disabled={!complete || elsewhere.length > 0 || busy}
      onclick={() => void submit()}
    >
      {statusLabel}にする
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-12);
  }
  .prow {
    display: grid;
    grid-template-columns: 7em minmax(0, 1fr);
    align-items: center;
    gap: var(--space-8);
  }
  .lbl {
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .detail {
    color: var(--accent);
    font-size: var(--fs-13);
  }
  .note {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .error {
    margin: 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
