<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { RULE_EVENTS, WHEN_ALL_MERGED, whenLabel } from "../../core/automations";
  import { cliAutomationAdd, Placeholder } from "../../core/cli";
  import { eventLabel } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import { core } from "../context";
  import { useManyStores } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";

  // Adds a rule, as hh automation add: of the Store, or of a project when one is given.
  type Props = {
    store: string;
    schema: SchemaView;
    project?: { ulid: string; id: string };
    onclose: () => void;
  };
  let { store, schema, project, onclose }: Props = $props();

  const c = core();
  const many = useManyStores(c.stores);

  let id = $state("");
  let on = $state(RULE_EVENTS[0] ?? "");
  // status:<name> or category:<name>
  let goal = $state("");
  let all = $state(true);
  let busy = $state(false);
  let error = $state("");

  const categories = $derived([...new Set((schema.statuses ?? []).map((x) => x.category))]);
  const typed = $derived(id.trim());
  const sep = $derived(goal.indexOf(":"));
  const what = $derived(goal.slice(0, sep));
  const name = $derived(goal.slice(sep + 1));
  const status = $derived(what === "status" ? name : undefined);
  const category = $derived(what === "category" ? name : undefined);
  const when = $derived(on === "pr.merged" && all ? WHEN_ALL_MERGED : undefined);
  const ready = $derived(typed !== "" && goal !== "");
  const cli = $derived(
    cliAutomationAdd({
      id: typed || new Placeholder("id"),
      on,
      status,
      category,
      when,
      project: project?.id,
      store: many.current ? store : undefined,
    }),
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!ready || busy) return;
    busy = true;
    error = "";
    try {
      await unwrap(
        c.api.POST("/api/automations", {
          params: { query: { store } },
          body: { id: typed, on, status, category, when, project: project?.ulid },
        }),
      );
      onclose();
    } catch (err) {
      error = errorMessage(err);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog
  open
  title={project ? "このプロジェクトだけのルールを追加" : "ルールを追加"}
  width={560}
  {cli}
  cliDisabled={!ready}
  {onclose}
>
  <form id="rule-form" class="form" onsubmit={submit}>
    <label class="field">
      <span>id</span>
      <input class="mono" autocomplete="off" placeholder="pr-opened" bind:value={id}>
    </label>
    <label class="field">
      <span>イベント</span>
      <select bind:value={on}>
        {#each RULE_EVENTS as e (e)}
          <option value={e}>{eventLabel(e)}</option>
        {/each}
      </select>
    </label>
    {#if on === "pr.merged"}
      <label class="check">
        <input type="checkbox" bind:checked={all}>
        {whenLabel(WHEN_ALL_MERGED)}のときだけ
      </label>
    {/if}
    <label class="field">
      <span>行き先</span>
      <select bind:value={goal}>
        <option value="" disabled>選んでください</option>
        <optgroup label="ステータス">
          {#each schema.statuses ?? [] as s (s.name)}
            <option value="status:{s.name}">{s.label || s.name}</option>
          {/each}
        </optgroup>
        <optgroup label="カテゴリの既定">
          {#each categories as cat (cat)}
            <option value="category:{cat}">{cat}</option>
          {/each}
        </optgroup>
      </select>
    </label>
    {#if error}
      <FormError message={error} />
    {/if}
  </form>
  {#snippet footer()}
    <Button disabled={busy} onclick={onclose}>キャンセル</Button>
    <Button variant="confirm" type="submit" form="rule-form" disabled={busy || !ready}>追加</Button>
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
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  .field input,
  .field select {
    min-height: var(--control-h);
    padding: 0 var(--space-8);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-weight: 400;
  }
  .field .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .check {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    font-size: var(--fs-12);
    color: var(--ink-2);
  }
</style>
