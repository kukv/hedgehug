<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { projectRuleRow } from "../../core/automations";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import { core } from "../context";
  import { useRules } from "../github.svelte";
  import RuleChips from "../settings/RuleChips.svelte";
  import RuleDialog from "../settings/RuleDialog.svelte";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";

  // The rules in a project: stop one of the Store here, add one for this project alone. Each change
  // writes at once, apart from the draft of the rest of the settings.
  type Props = {
    store: string;
    schema: SchemaView;
    project: { ulid: string; id: string };
    off: string | null;
  };
  let { store, schema, project, off }: Props = $props();

  const c = core();
  const inProject = useRules(
    () => store,
    () => project.ulid,
  );
  const whole = useRules(() => store);

  let adding = $state(false);
  let busy = $state(false);

  const rows = $derived(
    (inProject.current ?? []).map((rule) => ({
      rule,
      row: projectRuleRow(
        rule,
        whole.current?.find((r) => r.id === rule.id),
        project.ulid,
        store,
      ),
    })),
  );

  async function run(write: () => Promise<unknown>, undo: () => void) {
    busy = true;
    try {
      await write();
    } catch (e) {
      undo();
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }

  const toggle = (id: string, scoped: string | undefined, el: HTMLInputElement) => {
    const enabled = el.checked;
    return run(
      () =>
        unwrap(
          c.api.PUT("/api/automations/{id}/enabled", {
            params: { path: { id }, query: { store } },
            body: { enabled, project: scoped },
          }),
        ),
      () => {
        el.checked = !enabled;
      },
    );
  };

  const remove = (id: string) =>
    run(
      () =>
        unwrap(
          c.api.DELETE("/api/automations/{id}", {
            params: { path: { id }, query: { store } },
          }),
        ),
      () => undefined,
    );
</script>

<section class="block" aria-labelledby="project-rules-h">
  <div class="head">
    <h2 id="project-rules-h">自動化</h2>
    <span class="msg"
      >{store}
      のルールをこのプロジェクトで止める・このプロジェクトだけのルールを足す</span
    >
  </div>
  <div class="rows">
    {#each rows as { rule, row } (rule.id)}
      <div class="row" data-rule={rule.id}>
        <RuleChips {rule} {schema} />
        <code>{rule.id}</code>
        <span class="end">
          <span class="source" class:own={row.own}>{row.source}</span>
          <label class="switch">
            <input
              type="checkbox"
              aria-label="{row.state}: {rule.id}"
              checked={row.checked}
              disabled={busy || row.fixed || off !== null}
              onchange={(e) => void toggle(rule.id, row.project, e.currentTarget)}
            >
            {row.state}
          </label>
          {#if row.own}
            <Button
              size="sm"
              variant="ghost"
              aria-label="削除: {rule.id}"
              disabled={busy || off !== null}
              why={off ?? undefined}
              onclick={() => void remove(rule.id)}
            >
              削除
            </Button>
          {/if}
        </span>
      </div>
    {:else}
      <p class="msg none">ルールはありません</p>
    {/each}
  </div>
  <div>
    <Button
      size="sm"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => (adding = true)}
    >
      このプロジェクトだけのルールを追加
    </Button>
  </div>
</section>

{#if adding}
  <RuleDialog {store} {schema} {project} onclose={() => (adding = false)} />
{/if}

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-20) 0;
    border-bottom: 1px solid var(--line);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-10);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .msg {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .rows {
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-6) var(--space-12);
  }
  .row + .row {
    border-top: 1px solid var(--line);
  }
  code {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
  .end {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-10);
    margin-left: auto;
  }
  .source {
    padding: 1px var(--space-8);
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-11);
  }
  .source.own {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .switch {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .none {
    display: block;
    margin: 0;
    padding: var(--space-8) var(--space-12);
  }
</style>
