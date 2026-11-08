<script lang="ts" module>
  export type MigrateTarget = "status" | "kind" | "prop";

  // How long the name has to rest before the dry run is sent.
  const PAUSE_MS = 350;

  const NOUN: Record<MigrateTarget, string> = {
    status: "ステータス",
    kind: "種類",
    prop: "プロパティ",
  };
</script>

<script lang="ts">
  import { tick } from "svelte";
  import { cliSchemaMigrate, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Merges a definition of the schema into another, and rewrites the tasks that use it (a name is
  // changed in the edit dialog). What the run would do is asked for first (dry_run) when the name stops changing.
  // names are all the definitions of its kind, old among them.
  type Props = {
    store: string;
    many: boolean;
    target: MigrateTarget;
    old: string;
    names: string[];
    onclose: () => void;
    ondone: () => void;
  };
  let { store, many, target, old, names, onclose, ondone }: Props = $props();

  const c = core();
  const uid = $props.id();

  let next = $state("");
  // What the dry run said for the name in the field: null until it is in.
  let preview = $state.raw<{ name: string; changed: number; skipped: number } | null>(null);
  let previewFailure = $state<string | null>(null);
  let busy = $state(false);
  let failure = $state<string | null>(null);
  // The latest dry run wins: an answer of an older one, or one after the dialog is gone, is dropped.
  let seq = 0;
  let form: HTMLElement | undefined = $state();

  const name = $derived(next.trim());
  const sendable = $derived(name !== "" && name !== old);
  const others = $derived(names.filter((n) => n !== old));
  const ready = $derived(!busy && preview !== null && preview.name === name);

  const body = $derived({ target, old, new: name });

  $effect(() => {
    const n = name;
    const send = sendable;
    preview = null;
    previewFailure = null;
    if (!send) return;
    const timer = setTimeout(() => void dry(n, ++seq), PAUSE_MS);
    return () => {
      clearTimeout(timer);
      seq++;
    };
  });

  async function dry(n: string, mine: number): Promise<void> {
    try {
      const r = await c.api.migrateSchema({ ...body, new: n, dry_run: true }, { store });
      if (mine !== seq) return;
      preview = { name: n, changed: (r.changed ?? []).length, skipped: (r.skipped ?? []).length };
    } catch (e) {
      if (mine === seq) previewFailure = errorMessage(e);
    }
  }

  // The write is done once it is sent: its toast and the reload stand even if the dialog was left.
  async function run(): Promise<void> {
    if (!ready) return;
    busy = true;
    failure = null;
    try {
      const r = await c.api.migrateSchema(body, { store });
      await c.schema.reload(store);
      const skipped = (r.skipped ?? []).length;
      toast({
        message: `${(r.changed ?? []).length} 件を書き換えました${
          skipped > 0 ? `（読み込めないため書き換えなかったタスク ${skipped} 件）` : ""
        }`,
      });
      ondone();
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
    // The button the user pressed was disabled while the run was on, and a disabled control drops the focus.
    if (failure !== null) {
      await tick();
      form?.querySelector<HTMLElement>("[data-autofocus]")?.focus();
    }
  }

  function close(): void {
    if (!busy) onclose();
  }

  function onkeydown(e: KeyboardEvent): void {
    if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229 && ready) {
      e.preventDefault();
      void run();
    }
  }

  const title = $derived(`${NOUN[target]}「${old}」をマージ`);
  const cli = $derived(
    cliSchemaMigrate({
      ...body,
      new: name === "" ? new Placeholder("マージ先") : name,
      store: many ? store : undefined,
    }),
  );
  const mergeNote = $derived(`「${old}」は定義から消え、「${name}」にマージされます`);
  const summary = $derived.by(() => {
    if (preview !== null) {
      return preview.changed === 0
        ? "書き換わるタスクはありません"
        : `${preview.changed} 件のタスクが書き換わります`;
    }
    return sendable && previewFailure === null ? "確認しています…" : "";
  });
  const skippedNote = $derived(
    preview !== null && preview.skipped > 0
      ? `読み込めないため書き換えないタスク ${preview.skipped} 件`
      : "",
  );
</script>

<Dialog open {title} width={560} {cli} cliDisabled={name === ""} onclose={close}>
  <div class="form" role="presentation" bind:this={form} {onkeydown}>
    <div class="field">
      <label class="lbl" for="{uid}-to">マージ先</label>
      <select id="{uid}-to" data-autofocus disabled={busy} bind:value={next}>
        <option value="">選んでください</option>
        {#each others as n (n)}
          <option value={n}>{n}</option>
        {/each}
      </select>
    </div>
    {#if sendable}
      <p class="note">{mergeNote}</p>
    {/if}
    <p class="sum" aria-live="polite">{summary}</p>
    {#if skippedNote}
      <p class="note">{skippedNote}</p>
    {/if}
    {#if previewFailure}
      <FormError message={previewFailure} />
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={!ready} onclick={() => void run()}> マージ </Button>
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
  select {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-14);
  }
  select:focus-visible {
    outline: 2px solid var(--accent);
  }
  .sum {
    margin: 0;
    min-height: 1.5em;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.6;
  }
</style>
