<script lang="ts">
  import { tick } from "svelte";
  import type { SchemaView } from "../../core/schema";
  import { orphanedBy, referencesOf } from "../../core/schema-edit";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import { failureOf } from "./schema-text";

  // Deletes a definition of the schema, and says first what else
  // the removal takes out. projectLabel is how a project of an override is shown (its ID and title).
  type Props = {
    store: string;
    target: "status" | "kind" | "prop";
    name: string;
    schema: SchemaView;
    projectLabel: (ulid: string) => string;
    onclose: () => void;
    ondone: () => void;
  };
  let { store, target, name, schema, projectLabel, onclose, ondone }: Props = $props();

  const c = core();
  const uid = $props.id();
  const noun = $derived(
    target === "status"
      ? "ステータス"
      : target === "kind"
        ? "種類"
        : schema.props[name]?.type === "task_ref"
          ? "関連"
          : "プロパティ",
  );
  const what = $derived(`${noun}「${name}」`);
  const title = $derived(`${what}を削除`);
  const refs = $derived(referencesOf(schema, target, name));
  const orphans = $derived(target === "kind" ? orphanedBy(schema, name) : []);
  const orphanReason = $derived(
    orphans.length > 0
      ? `種類 ${orphans.join("、")} のどの下に置けるかがなくなるため削除できません。先に ${orphans.join("、")} のどの下に置けるかを変えてください`
      : "",
  );

  let busy = $state(false);
  let err = $state.raw<unknown>(null);
  let form: HTMLElement | undefined = $state();
  const failure = $derived(err === null ? null : failureOf(err).text);

  async function run(): Promise<void> {
    if (busy || orphans.length > 0) return;
    busy = true;
    err = null;
    const q = { store };
    try {
      if (target === "status") await c.api.deleteStatus(name, q);
      else if (target === "kind") await c.api.deleteKind(name, q);
      else await c.api.deleteProp(name, q);
      await c.schema.reload(store);
      toast({ message: `${what}を削除しました` });
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
</script>

<Dialog open {title} width={560} onclose={close}>
  <div class="form" bind:this={form}>
    {#if refs.length > 0}
      <p class="lead">次の参照も外れます</p>
      <ul class="refs">
        {#each refs as r (r.text + (r.project ?? ""))}
          <li>{r.text}{r.project ? `（${projectLabel(r.project)}）` : ""}</li>
        {/each}
      </ul>
    {:else}
      <p class="lead">{what}を定義から削除します。</p>
    {/if}
    {#if orphanReason}
      <p class="note" id="{uid}-orphan-why">{orphanReason}</p>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button
      variant="danger"
      data-autofocus
      disabled={busy || orphans.length > 0}
      aria-describedby={orphans.length > 0 ? `${uid}-orphan-why` : undefined}
      onclick={() => void run()}
    >
      削除
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .lead {
    margin: 0;
    font-size: var(--fs-13);
  }
  .refs {
    margin: 0;
    padding-left: var(--space-20, 20px);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .note {
    margin: 0;
    color: var(--danger-ink);
    font-size: var(--fs-12);
    line-height: 1.6;
  }
</style>
