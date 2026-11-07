<script lang="ts">
  import { tick, untrack } from "svelte";
  import { cliDoctor } from "../../core/cli";
  import {
    type DoctorReport,
    findingHint,
    findingText,
    fixedText,
    groupFindings,
    type Severity,
    severityOf,
  } from "../../core/doctor";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon, { type IconName } from "../icons/Icon.svelte";
  import { useReadOnlySentence } from "../read-only";
  import { onLinkClick, router } from "../router";
  import { useDisplayId } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";

  // The diagnosis (hh doctor) of one Store, and its repair (hh doctor --fix). It is asked for when the
  // section is drawn and when the user asks again: a change of the tasks does not ask, the check is heavy.
  let { store, many }: { store: string; many: boolean } = $props();

  const c = core();
  const did = useDisplayId(c.stores);
  const uid = $props.id();
  // The repair writes; the diagnosis only reads (細部 35).
  const offSentence = useReadOnlySentence(c.stores, () => store);
  const off = $derived(offSentence.current);

  const SEVERITY: Record<Severity, { label: string; icon: IconName }> = {
    error: { label: "エラー", icon: "forbid" },
    warning: { label: "警告", icon: "warning" },
    info: { label: "情報", icon: "status" },
  };

  let report = $state.raw<DoctorReport | null>(null);
  // The ID of each task a finding is about, by ULID.
  let ids = $state.raw<Record<string, string>>({});
  let failure = $state<string | null>(null);
  let loading = $state(false);
  let fixing = $state(false);
  let dialogOpen = $state(false);
  // What the last repair did (the findings that are left are the report's).
  let fixed = $state.raw<DoctorReport | null>(null);
  let resultEl: HTMLElement | undefined = $state();
  let root: HTMLElement | undefined = $state();
  // The latest request wins: an answer of an older one, or one that comes after the section is gone, is dropped.
  let seq = 0;

  const report_for = (rs: DoctorReport[] | null | undefined, s: string) =>
    (rs ?? []).find((r) => r.store === s) ?? null;

  // The ID a task has, from the cache or from the task itself. A task that cannot be read is opened
  // without its ID ("タスクを開く"): the link works either way, so this failure is not shown.
  async function resolveIds(r: DoctorReport, s: string, mine: number): Promise<void> {
    const ulids = [...new Set((r.findings ?? []).flatMap((f) => (f.task ? [f.task] : [])))];
    const found = await Promise.all(
      ulids.map(async (ulid): Promise<[string, string] | null> => {
        const cached = c.cache.task(ulid)?.id;
        if (cached) return [ulid, cached];
        try {
          const t = await c.api.getTask(ulid, { store: s });
          return t.id ? [ulid, t.id] : null;
        } catch {
          return null;
        }
      }),
    );
    if (mine !== seq) return;
    ids = Object.fromEntries(found.filter((x): x is [string, string] => x !== null));
  }

  async function load(s: string): Promise<void> {
    const mine = ++seq;
    loading = true;
    try {
      const rs = await c.api.doctor({ store: s });
      if (mine !== seq) return;
      const r = report_for(rs, s);
      if (!r) throw new Error(`${s} の診断の結果が返りませんでした`);
      report = r;
      failure = null;
      // The buttons do not wait for the IDs: they fill in when they come.
      loading = false;
      void resolveIds(r, s, mine);
    } catch (e) {
      // The result that was read stays; the failure is shown above it.
      if (mine === seq) failure = errorMessage(e);
    } finally {
      if (mine === seq) loading = false;
    }
  }

  $effect(() => {
    const s = store;
    untrack(() => void load(s));
    return () => {
      seq++;
    };
  });

  async function fix(): Promise<void> {
    if (fixing || loading || off !== null) return;
    const s = store;
    const mine = ++seq;
    fixing = true;
    try {
      const rs = await c.api.doctorFix({ store: s });
      if (mine !== seq) return;
      const r = report_for(rs, s);
      if (!r) throw new Error(`${s} の修復の結果が返りませんでした`);
      // The report of a repair is the diagnosis after it: it is what is left.
      report = r;
      fixed = r;
      failure = null;
      dialogOpen = false;
      const n = r.repaired?.length ?? 0;
      toast({
        message:
          n > 0
            ? `${n} 件を修復しました`
            : r.rebuilt
              ? "Store を origin の新しい履歴にそろえました"
              : "修復するものはありませんでした",
      });
      // 「修復…」 may be gone now: the focus goes to the result.
      await tick();
      resultEl?.focus();
      fixing = false;
      await resolveIds(r, s, mine);
    } catch (e) {
      if (mine === seq) toast({ message: errorMessage(e) });
    } finally {
      if (mine === seq) fixing = false;
    }
  }

  function closeDialog(): void {
    if (!fixing) dialogOpen = false;
  }

  async function again(): Promise<void> {
    if (loading || fixing) return;
    fixed = null;
    await load(store);
    // The button was disabled while it asked: the focus comes back to it.
    await tick();
    root?.querySelector<HTMLElement>("[data-again]")?.focus();
  }

  const groups = $derived(groupFindings(report?.findings ?? []));
  const findingsCount = $derived(report?.findings?.length ?? 0);
  const summary = $derived.by(() => {
    const n: Record<Severity, number> = { error: 0, warning: 0, info: 0 };
    for (const f of report?.findings ?? []) n[severityOf(f)]++;
    return (["error", "warning", "info"] as const)
      .filter((k) => n[k] > 0)
      .map((k) => `${SEVERITY[k].label} ${n[k]} 件`)
      .join(" · ");
  });
  const cli = $derived(cliDoctor({ fix: true, store: many ? store : undefined }));
</script>

<section class="store" aria-labelledby="{uid}-h" bind:this={root}>
  <h3 id="{uid}-h" class="sr">診断: {store}</h3>
  <div class="bar">
    {#if many}
      <StoreBadge {store} tone={c.stores.tone(store)} />
    {/if}
    <span class="sum" aria-live="polite">
      {#if report === null}
        {loading ? "診断しています…" : ""}
      {:else if findingsCount === 0}
        問題は見つかりませんでした
      {:else}
        {summary}
      {/if}
    </span>
    <span class="end">
      <Button size="sm" data-again disabled={loading || fixing} onclick={() => void again()}>
        もう一度実行
      </Button>
      {#if findingsCount > 0}
        <Button
          size="sm"
          disabled={loading || fixing || off !== null}
          why={off ?? undefined}
          onclick={() => (dialogOpen = true)}
        >
          修復…
        </Button>
      {/if}
    </span>
  </div>

  {#if failure}
    <FormError message={failure} />
  {/if}

  {#if fixed}
    <section class="result" aria-labelledby="{uid}-res" tabindex="-1" bind:this={resultEl}>
      <h4 id="{uid}-res">修復の結果</h4>
      {#if fixed.rebuilt}
        <p class="line">origin の新しい履歴にそろえました</p>
        <p class="line">未 push の {fixed.rebuilt.replayed} コミットを再適用しました</p>
        {#if (fixed.rebuilt.dropped ?? []).length > 0}
          <p class="line">戻せなかったタスク: {(fixed.rebuilt.dropped ?? []).join(", ")}</p>
        {/if}
        {#if fixed.rebuilt.kept}
          <p class="line">
            古い Store は <span class="mono">{fixed.rebuilt.kept}</span> にあります
          </p>
        {/if}
      {/if}
      {#if (fixed.repaired ?? []).length > 0}
        <ul class="fixed">
          {#each fixed.repaired ?? [] as x (`${x.ulid}:${x.kind}`)}
            <li>
              {#if x.id}
                <span class="mono">{did.current(store, x.id)}</span>
              {/if}
              {#if x.title}
                <span>{x.title}</span>
              {/if}
              <span class="why">{fixedText(x.kind, x.value)}</span>
            </li>
          {/each}
        </ul>
      {:else if !fixed.rebuilt}
        <p class="line">修復するものはありませんでした</p>
      {/if}
    </section>
  {/if}

  {#each groups as g (g.check)}
    <div class="group">
      <h4>{g.name} <span class="n">{g.items.length}</span></h4>
      <ul class="findings">
        {#each g.items as f}
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
                {@const id = ids[f.task]}
                <a class="open" {href} onclick={(e) => onLinkClick(e, href)}>
                  {id ? `${did.current(store, id)} を開く` : "タスクを開く"}
                </a>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    </div>
  {/each}
  <div><CliLine command={cliDoctor({ store: many ? store : undefined })} /></div>
</section>

<Dialog open={dialogOpen} title="doctor で修復する" width={560} {cli} onclose={closeDialog}>
  <p class="p">{many ? `Store ${store} の` : ""}次の問題を、1 コミットで直します。</p>
  <ul class="kinds">
    <li>親の循環</li>
    <li>削除済みの親の子</li>
    <li>親と違うワークスペースにあるタスク</li>
    <li>重複した ID</li>
  </ul>
  <p class="p">
    force push の後は、Store を origin の新しい履歴にそろえ、未 push
    の変更を再適用します。ほかの問題は直せないので、診断に残ります。
  </p>
  {#snippet footer()}
    <Button onclick={closeDialog} disabled={fixing}>キャンセル</Button>
    <Button variant="danger" onclick={() => void fix()} disabled={fixing}>修復する</Button>
  {/snippet}
</Dialog>

<style>
  .store {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
  }
  .sum {
    font-size: var(--fs-13);
    color: var(--ink-2);
  }
  .end {
    display: flex;
    gap: var(--space-8);
    margin-left: auto;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  h4 {
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .n {
    color: var(--muted);
    font-weight: 400;
  }
  .findings,
  .fixed {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .finding {
    display: flex;
    gap: var(--space-10);
    font-size: var(--fs-13);
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
  .result {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--sunken);
    font-size: var(--fs-13);
  }
  .result:focus-visible {
    outline: 2px solid var(--accent);
  }
  .line {
    margin: 0;
  }
  .fixed li {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }
  .why {
    color: var(--muted);
  }
  .p {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .kinds {
    margin: 0 0 var(--space-8);
    padding-left: var(--space-20);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
</style>
