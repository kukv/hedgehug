<script lang="ts">
  import FieldInput from "./FieldInput.svelte";
  import type { Option } from "./field-types";
  import type { PropRow } from "./prop-rows";

  // The editor of one property of the schema, by the type of the property. The detail panel and
  // the create dialog share it. The caller leaves out the rows without an editor (properties the
  // schema does not define); the text editor is the fallback for the rest.
  let {
    row,
    value,
    pending = false,
    readonly = false,
    describedby,
    onedit,
    oncommit,
  }: {
    row: PropRow;
    value: unknown;
    pending?: boolean;
    readonly?: boolean;
    describedby?: string;
    onedit: (v: unknown) => void;
    oncommit: () => void;
  } = $props();

  const boolOptions: Option[] = [
    { value: "true", label: "はい" },
    { value: "false", label: "いいえ" },
  ];
  const options = $derived(row.options.map((o) => ({ value: o, label: o })));
  // A bool is shown as a select of strings and sent as a boolean.
  const boolValue = $derived(value === null || value === undefined ? null : String(value));
</script>

{#if row.editor === "enum" && !row.multi}
  <FieldInput
    editor="select"
    label={row.label}
    {value}
    {readonly}
    {describedby}
    {options}
    {onedit}
    {oncommit}
  />
{:else if row.editor === "enum" ||
  row.editor === "multi_enum" ||
  (row.editor === "user" && row.multi)}
  <FieldInput
    editor="chips"
    label={row.label}
    {value}
    {readonly}
    {describedby}
    {options}
    {onedit}
    {oncommit}
  />
{:else if row.editor === "bool"}
  <FieldInput
    editor="select"
    label={row.label}
    value={boolValue}
    {readonly}
    {describedby}
    options={boolOptions}
    onedit={(v) => onedit(v === null ? null : v === "true")}
    {oncommit}
  />
{:else if row.editor === "date"}
  <FieldInput
    editor="date"
    label={row.label}
    {value}
    {readonly}
    {describedby}
    {onedit}
    {oncommit}
  />
{:else if row.editor === "number"}
  <FieldInput
    editor="number"
    label={row.label}
    {value}
    {readonly}
    {describedby}
    {pending}
    {onedit}
    {oncommit}
  />
{:else}
  <FieldInput
    editor="text"
    label={row.label}
    {value}
    {readonly}
    {describedby}
    {pending}
    {onedit}
    {oncommit}
  />
{/if}
