<script lang="ts">
  import { untrack } from "svelte";
  import { unwrap } from "../../core/api/client";
  import { cliSprintStart } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { ptLabel, type Sprint } from "../../core/planning";
  import type { WorkspaceEntry } from "../../core/stores";
  import { core } from "../context";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import PlanField from "./PlanField.svelte";
  import { plansWritten } from "./plans.svelte";

  type Props = { entry: WorkspaceEntry; sprint: Sprint; sprints: Sprint[]; onclose: () => void };
  let { entry, sprint, sprints, onclose }: Props = $props();

  const c = core();
  const was = untrack(() => sprint);
  let name = $state(was.name);
  let start = $state(was.start ?? "");
  let end = $state(was.end ?? "");
  let goal = $state(was.goal ?? "");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const others = $derived(sprints.filter((s) => s.state === "active" && s.ulid !== sprint.ulid));
  const changes = $derived.by(() => {
    const out: Record<string, string> = {};
    if (name.trim() !== was.name) out.name = name.trim();
    if (start !== (was.start ?? "")) out.start = start;
    if (end !== (was.end ?? "")) out.end = end;
    if (goal !== (was.goal ?? "")) out.goal = goal;
    return out;
  });
  const invalid = $derived(name.trim() === "" || start === "" || end === "");

  async function submit() {
    if (invalid || busy) return;
    busy = true;
    failure = null;
    const params = { path: { id: was.ulid }, query: { store: entry.store } };
    try {
      if (Object.keys(changes).length > 0) {
        await unwrap(c.api.PATCH("/api/sprints/{id}", { params, body: changes }));
      }
      await unwrap(c.api.POST("/api/sprints/{id}/start", { params }));
      plansWritten();
      toast({ message: `スプリント ${name.trim()} を開始しました` });
      onclose();
    } catch (e) {
      failure = errorMessage(e);
      plansWritten();
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open
  title="スプリントを開始"
  width={620}
  cli={cliSprintStart(was.name, entry.store)}
  {onclose}
>
  <div class="form">
    <div class="row">
      <PlanField label="名前" value={name} first oninput={(v) => (name = v)} />
      <PlanField label="start" type="date" value={start} oninput={(v) => (start = v)} />
      <PlanField label="end" type="date" value={end} oninput={(v) => (end = v)} />
    </div>
    <PlanField label="ゴール" value={goal} oninput={(v) => (goal = v)} />
    <p class="load">入っているタスク {sprint.tasks} 件 · {ptLabel(sprint.points)}</p>
    {#if others.length > 0}
      <p class="note" role="note">
        ワークスペース {entry.name} では {others.map((s) => s.name).join("、")} も active です。1
        つのワークスペースで複数のスプリントを同時に active にできます
      </p>
    {/if}
    {#if failure}
      <FormError message={failure} />
    {/if}
  </div>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy || invalid} onclick={() => void submit()}>
      開始する
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
  .load {
    margin: 0;
    padding: var(--space-12);
    border-radius: var(--radius-8);
    background: var(--sunken);
    font-weight: 600;
  }
  .note {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-8);
    background: var(--accent-weak);
    line-height: 1.6;
  }
</style>
