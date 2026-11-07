<script lang="ts">
  import { untrack } from "svelte";
  import { cliSprintAdd, cliSprintSet, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { Sprint } from "../../core/planning";
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

  type Props = { entry?: WorkspaceEntry; sprint?: Sprint; onclose: () => void };
  let { entry, sprint, onclose }: Props = $props();

  const c = core();
  const all = useSubscribe(c.stores, () => c.stores.workspaces());
  let chosen = $state.raw<WorkspaceEntry | undefined>(untrack(() => entry));
  const target = $derived(entry ?? chosen);

  const was = untrack(() => sprint);
  let name = $state(was?.name ?? "");
  let start = $state(was?.start ?? "");
  let end = $state(was?.end ?? "");
  let goal = $state(was?.goal ?? "");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const changes = $derived.by(() => {
    if (!was) return {};
    const out: Record<string, string> = {};
    if (name.trim() !== was.name) out.name = name.trim();
    if (start !== (was.start ?? "")) out.start = start;
    if (end !== (was.end ?? "")) out.end = end;
    if (goal !== (was.goal ?? "")) out.goal = goal;
    return out;
  });
  const invalid = $derived(
    name.trim() === "" || !target || (was !== undefined && Object.keys(changes).length === 0),
  );
  const cli = $derived.by(() => {
    const store = target?.store ?? "";
    if (was) return cliSprintSet(was.name, changes, store);
    return cliSprintAdd({
      name: name.trim() === "" ? new Placeholder("名前") : name.trim(),
      start,
      end,
      goal,
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
        await c.api.updateSprint(was.ulid, changes, { store });
      } else {
        await c.api.addSprint(
          {
            name: name.trim(),
            workspace: target.name,
            ...(start ? { start } : {}),
            ...(end ? { end } : {}),
            ...(goal ? { goal } : {}),
          },
          { store },
        );
      }
      await c.stores.reload().catch(() => undefined);
      plansWritten();
      toast({ message: `スプリント ${name.trim()} を${was ? "変更" : "作成"}しました` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open title={was ? "スプリントを編集" : "スプリントを作成"} width={620} {cli} {onclose}>
  <div class="form">
    {#if !entry}
      <WorkspaceSelect entries={all.current} onchange={(w) => (chosen = w)} />
    {/if}
    <div class="row">
      <PlanField label="名前" value={name} first oninput={(v) => (name = v)} />
      <PlanField label="開始日" type="date" value={start} oninput={(v) => (start = v)} />
      <PlanField label="終了日" type="date" value={end} oninput={(v) => (end = v)} />
    </div>
    <PlanField label="ゴール" value={goal} oninput={(v) => (goal = v)} />
    {#if !was}
      <p class="note">状態は planned で作ります · 開始するには開始日と終了日が必要です</p>
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
