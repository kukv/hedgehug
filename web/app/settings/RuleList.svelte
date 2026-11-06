<script lang="ts">
  import { requiredNote, ruleTarget, ruleYaml, statLabel } from "../../core/automations";
  import { cliAutomationRm } from "../../core/cli";
  import type { Rule, RuleStat } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import { core } from "../context";
  import { requiredFields, requiredLabel } from "../kanban/drop";
  import { hrefOf, onLinkClick } from "../router";
  import { useManyStores } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import RuleChips from "./RuleChips.svelte";

  // The rules of a Store in the order they apply. A switch writes at once; the rule comes back
  // through automations.changed.
  type Props = {
    store: string;
    rules: Rule[];
    stats: RuleStat[];
    schema: SchemaView;
    off: string | null;
  };
  let { store, rules, stats, schema, off }: Props = $props();

  const c = core();
  const many = useManyStores(c.stores);

  let removing = $state.raw<Rule | null>(null);
  let removeError = $state("");
  let busy = $state(false);

  const statOf = (r: Rule) => stats.find((s) => s.rule === r.id);
  const noteOf = (r: Rule) =>
    requiredNote(
      r,
      schema,
      requiredFields(schema, r.project?.ulid, ruleTarget(r, schema)).map((f) =>
        requiredLabel(schema, f),
      ),
      statOf(r),
    );
  const projectHref = (ulid: string) => hrefOf({ name: "project", ulid, tab: "settings" });

  async function toggle(r: Rule, el: HTMLInputElement) {
    const enabled = el.checked;
    busy = true;
    try {
      await c.api.setAutomationEnabled(r.id, { enabled }, { store });
    } catch (e) {
      el.checked = r.enabled;
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }

  async function remove() {
    const r = removing;
    if (!r || busy) return;
    busy = true;
    removeError = "";
    try {
      await c.api.deleteAutomation(r.id, { store });
      removing = null;
    } catch (e) {
      removeError = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

{#each rules as r (r.id)}
  {@const note = noteOf(r)}
  <article class="rule" class:off={!r.enabled} aria-label={r.id}>
    <div class="line">
      <RuleChips rule={r} {schema} />
      <code>{ruleYaml(r)}</code>
      {#each r.disabled_in as p (p.ulid)}
        <a
          class="scope"
          href={projectHref(p.ulid)}
          onclick={(e) => onLinkClick(e, projectHref(p.ulid))}
        >
          {p.title}
          で停止
        </a>
      {/each}
      {#if r.project}
        {@const p = r.project}
        <a
          class="scope only"
          href={projectHref(p.ulid)}
          onclick={(e) => onLinkClick(e, projectHref(p.ulid))}
        >
          {p.title}
          だけ
        </a>
      {/if}
      <span class="end">
        <span class="stat">{statLabel(r, statOf(r))}</span>
        <label class="switch">
          <input
            type="checkbox"
            aria-label="有効: {r.id}"
            checked={r.enabled}
            disabled={busy || off !== null}
            onchange={(e) => void toggle(r, e.currentTarget)}
          >
          有効
        </label>
        <Button
          size="sm"
          variant="ghost"
          aria-label="削除: {r.id}"
          disabled={busy || off !== null}
          why={off ?? undefined}
          onclick={() => {
            removeError = "";
            removing = r;
          }}
        >
          削除
        </Button>
      </span>
    </div>
    {#if note}
      <p class="note">{note}</p>
    {/if}
  </article>
{:else}
  <p class="none">ルールはありません</p>
{/each}

<Dialog
  open={removing !== null}
  title="ルール {removing?.id ?? ""} を削除"
  width={560}
  cli={removing ? cliAutomationRm(removing.id, many.current ? store : undefined) : undefined}
  onclose={() => {
    if (!busy) removing = null;
  }}
>
  <p class="confirm">このルールを消します。プロジェクトごとの停止からも外れます（1 コミット）。</p>
  {#if removeError}
    <FormError message={removeError} />
  {/if}
  {#snippet footer()}
    <Button disabled={busy} onclick={() => (removing = null)}>やめる</Button>
    <Button variant="danger" disabled={busy} onclick={() => void remove()}>削除</Button>
  {/snippet}
</Dialog>

<style>
  .rule {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-8) var(--space-12);
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
  }
  .rule.off {
    border-style: dashed;
    background: var(--sunken);
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
  }
  code {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
    overflow-wrap: anywhere;
  }
  .scope {
    padding: 1px var(--space-8);
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-11);
    text-decoration: none;
  }
  .scope.only {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .end {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    margin-left: auto;
  }
  .stat,
  .none {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .none {
    margin: 0;
  }
  .switch {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .note {
    margin: 0;
    padding: var(--space-8) var(--space-10);
    border-radius: var(--radius-8);
    background: var(--warning-weak);
    color: var(--warning);
    font-size: var(--fs-12);
    line-height: 1.6;
  }
  .confirm {
    margin: 0 0 var(--space-12);
    line-height: 1.6;
  }
</style>
