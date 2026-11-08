<script lang="ts">
  import { tick } from "svelte";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { failureOf } from "./schema-text";

  // Adds a value to an enum or multi_enum property.
  type Props = { store: string; prop: string; onclose: () => void; ondone: () => void };
  let { store, prop, onclose, ondone }: Props = $props();

  const c = core();
  const uid = $props.id();

  let value = $state("");
  let busy = $state(false);
  let err = $state.raw<unknown>(null);
  let form: HTMLElement | undefined = $state();
  const failure = $derived(err === null ? null : failureOf(err).text);
  const next = $derived(value.trim());

  async function save(): Promise<void> {
    if (busy || next === "") return;
    busy = true;
    err = null;
    try {
      await c.api.addValue(prop, { value: next }, { store });
      await c.schema.reload(store);
      toast({ message: `${prop} に値「${next}」を追加しました` });
      ondone();
      onclose();
    } catch (e) {
      err = e;
    } finally {
      busy = false;
    }
    if (err !== null) {
      await tick();
      form?.querySelector<HTMLElement>("[data-autofocus]")?.focus();
    }
  }

  function close(): void {
    if (!busy) onclose();
  }

  function onkeydown(e: KeyboardEvent): void {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229) {
      e.preventDefault();
      void save();
    }
  }
</script>

<Dialog open title="{prop} に値を追加" width={560} onclose={close}>
  <div class="form" role="presentation" bind:this={form} {onkeydown}>
    <div class="field">
      <label class="lbl" for="{uid}-value">値</label>
      <input
        id="{uid}-value"
        type="text"
        autocomplete="off"
        data-autofocus
        disabled={busy}
        bind:value
      >
    </div>
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={busy || next === ""} onclick={() => void save()}
      >追加</Button
    >
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
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
  input[type="text"] {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-14);
  }
  input[type="text"]:focus-visible {
    outline: 2px solid var(--accent);
  }
</style>
