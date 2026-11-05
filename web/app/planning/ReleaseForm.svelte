<script lang="ts">
  import { untrack } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliReleaseAdd, cliReleaseSet, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { Release } from "../../core/planning";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import PlanField from "./PlanField.svelte";
  import { plansWritten } from "./plans.svelte";
  import WorkspaceSelect from "./WorkspaceSelect.svelte";

  type Props = { entry?: WorkspaceEntry; release?: Release; onclose: () => void };
  let { entry, release, onclose }: Props = $props();

  const c = core();
  const all = useSubscribe(c.stores, () => c.stores.workspaces());
  let chosen = $state.raw<WorkspaceEntry | undefined>(untrack(() => entry));
  const target = $derived(entry ?? chosen);

  const was = untrack(() => release);
  let name = $state(was?.name ?? "");
  let date = $state(was?.date ?? "");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const changes = $derived.by(() => {
    if (!was) return {};
    const out: Record<string, string> = {};
    if (name.trim() !== was.name) out.name = name.trim();
    if (date !== (was.date ?? "")) out.date = date;
    return out;
  });
  const invalid = $derived(
    name.trim() === "" || !target || (was !== undefined && Object.keys(changes).length === 0),
  );
  const cli = $derived.by(() => {
    const store = target?.store ?? "";
    if (was) return cliReleaseSet(was.name, changes, store);
    return cliReleaseAdd({
      name: name.trim() === "" ? new Placeholder("名前") : name.trim(),
      date,
      workspace: target?.name ?? "",
      store,
    });
  });

  async function submit() {
    if (invalid || busy || !target) return;
    busy = true;
    failure = null;
    try {
      const store = target.store;
      if (was) {
        await unwrap(
          c.api.PATCH("/api/releases/{id}", {
            params: { path: { id: was.ulid }, query: { store } },
            body: changes,
          }),
        );
      } else {
        await unwrap(
          c.api.POST("/api/releases", {
            params: { query: { store } },
            body: { name: name.trim(), workspace: target.name, ...(date ? { date } : {}) },
          }),
        );
      }
      await c.stores.reload().catch(() => undefined);
      plansWritten();
      toast({ message: `リリース ${name.trim()} を${was ? "変更" : "作成"}しました` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title={was ? "リリースを編集" : "リリースを作成"} width={560} {cli} {onclose}>
  <div class="form">
    {#if !entry}
      <WorkspaceSelect entries={all.current} onchange={(w) => (chosen = w)} />
    {/if}
    <div class="row">
      <PlanField label="名前" value={name} first oninput={(v) => (name = v)} />
      <PlanField label="予定日" type="date" value={date} oninput={(v) => (date = v)} />
    </div>
    {#if !was && target}
      <p class="note">
        状態は unreleased で作ります · ワークスペース {target.name} のリリースになります
      </p>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      {was ? "保存" : "作成"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-12);
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
</style>
