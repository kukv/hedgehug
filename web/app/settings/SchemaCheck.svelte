<script lang="ts">
  import { untrack } from "svelte";
  import type { Schemas } from "../../core/api/client";
  import { cliSchemaCheck } from "../../core/cli";
  import {
    findingHint,
    findingText,
    groupFindings,
    type Severity,
    severityOf,
  } from "../../core/doctor";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon, { type IconName } from "../icons/Icon.svelte";
  import { onLinkClick, router } from "../router";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";

  // What `hh schema check` finds in a Store: it reads the check when the Store, its schema or its tasks
  // change. idOf is the ID of a task by ULID (undefined while it is not known), for the links.
  let {
    store,
    many,
    idOf,
  }: { store: string; many: boolean; idOf: (ulid: string) => string | undefined } = $props();

  type Finding = Schemas.Finding;

  const c = core();
  const uid = $props.id();

  let findings = $state.raw<Finding[] | null>(null);
  // A reload that fails keeps what was read; the failure is shown above it.
  let failure = $state<string | null>(null);
  // The latest request wins: an older answer, or one after the section is gone, is dropped.
  let seq = 0;

  async function load(s: string): Promise<void> {
    const mine = ++seq;
    try {
      const r = await c.api.checkSchema({ store: s });
      if (mine !== seq) return;
      findings = r.findings ?? [];
      failure = null;
    } catch (e) {
      if (mine === seq) failure = errorMessage(e);
    }
  }

  /** Reads the check now, without waiting for the event of the change (the page just wrote). */
  export function reload(): void {
    void load(store);
  }

  $effect(() => {
    const s = store;
    findings = null;
    failure = null;
    untrack(() => void load(s));
    const again = () => void load(s);
    const offs = [
      c.conn.on("schema.changed", (e) => {
        if (e.store === s) again();
      }),
      c.conn.on("tasks.changed", (e) => {
        if (e.store === s) again();
      }),
      c.conn.on("resync", again),
    ];
    return () => {
      for (const off of offs) off();
      seq++;
    };
  });

  const SEVERITY: Record<Severity, { label: string; icon: IconName }> = {
    error: { label: "エラー", icon: "forbid" },
    warning: { label: "警告", icon: "warning" },
    info: { label: "情報", icon: "status" },
  };

  const groups = $derived(groupFindings(findings ?? []));
  const summary = $derived(
    findings === null
      ? failure === null
        ? "検査しています…"
        : ""
      : findings.length === 0
        ? "スキーマとタスクは合っています"
        : `${findings.length} 件の指摘があります`,
  );
</script>

<section class="sec" aria-labelledby="{uid}-h">
  <h2 id="{uid}-h">スキーマの検査</h2>
  {#if failure}
    <FormError message="検査の結果を読み込めません · {failure}" />
  {/if}
  <p class="sum" aria-live="polite">{summary}</p>
  {#each groups as g (g.check)}
    <div class="group">
      <h3>{g.name} <span class="n">{g.items.length}</span></h3>
      <ul class="findings">
        {#each g.items as f, i (i)}
          {@const sev = severityOf(f)}
          <li class="finding" data-severity={sev}>
            <span class="sev"
              ><Icon name={SEVERITY[sev].icon} size={14} />{SEVERITY[sev].label}</span
            >
            <div class="what">
              <p class="msg">{findingText(f)}</p>
              {#if f.hint}
                <p class="hint">{findingHint(f)}</p>
              {/if}
              <p class="where mono">{f.where}</p>
              {#if f.task}
                {@const href = router.href({ name: "task", ulid: f.task })}
                {@const id = idOf(f.task)}
                <a class="open" {href} onclick={(e) => onLinkClick(e, href)}>
                  {id ? `${id} を開く` : "タスクを開く"}
                </a>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    </div>
  {/each}
  <div><CliLine command={cliSchemaCheck(many ? store : undefined)} /></div>
</section>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .n {
    color: var(--muted);
    font-weight: 400;
  }
  .sum {
    margin: 0;
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .findings {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--fs-13);
  }
  .finding {
    display: flex;
    gap: var(--space-10);
  }
  .sev {
    display: inline-flex;
    flex: 0 0 5rem;
    align-items: baseline;
    gap: var(--space-4);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .finding[data-severity="error"] .sev {
    color: var(--danger);
  }
  .finding[data-severity="warning"] .sev {
    color: var(--warning);
  }
  .finding[data-severity="info"] .sev {
    color: var(--muted);
  }
  .what {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
  }
  .what p {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .hint,
  .where {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .open {
    align-self: flex-start;
    color: var(--accent);
    font-size: var(--fs-12);
  }
</style>
