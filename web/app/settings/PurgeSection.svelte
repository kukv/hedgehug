<script lang="ts">
  import { tick } from "svelte";
  import { ApiError, unwrap } from "../../core/api/client";
  import { cliPurge } from "../../core/cli";
  import { type PurgePlan, parsePurgeDays } from "../../core/doctor";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { readOnlySentence, useReadOnlyOf } from "../read-only";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import IdLabel from "../ui/IdLabel.svelte";

  // Removes, for good, the deleted tasks that are older than N days (hh purge). It shows what would go
  // first, and deletes exactly that: a change of the days or of the Store takes the plan away.
  let { stores }: { stores: string[] } = $props();

  const c = core();
  const uid = $props.id();

  let selected = $state("");
  const store = $derived(stores.includes(selected) ? selected : (stores[0] ?? ""));
  // Looking at what would go is a read; deleting it is not (細部 35).
  const roOf = useReadOnlyOf(c.stores);
  const why = $derived(roOf.current(store));
  const off = $derived(why === null ? null : readOnlySentence(why));
  let input = $state("90");
  const days = $derived(parsePurgeDays(input));

  // The plan that was checked, for the days and the Store that are in the fields now: a change of
  // either takes it away, and the fields do not change while a request is on its way.
  let plan = $state.raw<PurgePlan | null>(null);
  let failure = $state<string | null>(null);
  let checking = $state(false);
  let purging = $state(false);
  let dialogOpen = $state(false);
  let root: HTMLElement | undefined = $state();
  // The latest request wins; an answer of an older one, or one that comes after the section is gone, is dropped.
  let seq = 0;

  const busy = $derived(checking || purging);
  const many = $derived(stores.length > 1);
  const empty = $derived(plan !== null && plan.tasks.length === 0 && plan.forwards.length === 0);

  $effect(() => () => {
    seq++;
  });

  function forget(): void {
    plan = null;
    failure = null;
  }

  // The controls are disabled while a request is on its way, which drops the focus: it comes back
  // to the one that began the action.
  // The first of the selectors that is there takes it.
  async function refocus(...selectors: string[]): Promise<void> {
    await tick();
    for (const sel of selectors) {
      const el = root?.querySelector<HTMLElement>(sel);
      if (el) return el.focus();
    }
  }

  async function run(s: string, d: number): Promise<void> {
    const mine = ++seq;
    checking = true;
    failure = null;
    try {
      const p = await unwrap(
        c.api.GET("/api/purge", { params: { query: { older_than: `${d}d`, store: s } } }),
      );
      if (mine !== seq) return;
      plan = p;
    } catch (e) {
      if (mine === seq) {
        plan = null;
        failure = errorMessage(e);
      }
    } finally {
      if (mine === seq) checking = false;
    }
  }

  async function checkNow(from: "[data-days]" | "[data-check]"): Promise<void> {
    if (busy || days === null || !store) return;
    await run(store, days);
    await refocus(from);
  }

  async function purge(): Promise<void> {
    if (busy || off !== null || !plan || days === null) return;
    const [s, d, confirmed] = [store, days, plan];
    const mine = ++seq;
    purging = true;
    try {
      const r = await unwrap(
        c.api.POST("/api/purge", {
          params: { query: { store: s } },
          body: { older_than: `${d}d`, confirmed },
        }),
      );
      if (mine !== seq) return;
      dialogOpen = false;
      plan = null;
      // The check is disabled while a request is on its way: it can take the focus only after this.
      purging = false;
      const n = r.tasks.length;
      toast({
        message: `${n} 件のタスクを完全に削除しました${r.commit ? `（${r.commit.slice(0, 7)}）` : ""}`,
      });
      // 「完全に削除…」 is gone: the focus goes back to the check.
      await refocus("[data-check]");
    } catch (e) {
      if (mine !== seq) return;
      toast({ message: errorMessage(e) });
      if (e instanceof ApiError && e.type === "purge.changed") {
        // What was checked is not what is there now: look again.
        dialogOpen = false;
        purging = false;
        await run(s, d);
        await refocus("[data-delete]", "[data-check]");
      }
    } finally {
      if (mine === seq) purging = false;
    }
  }

  function closeDialog(): void {
    if (!purging) dialogOpen = false;
  }

  const cli = $derived(plan && days !== null ? cliPurge(days, many ? store : undefined) : "");
  const planLine = $derived(
    !plan
      ? ""
      : empty
        ? "消すものはありません"
        : `${plan.tasks.length} 件のタスクと ${plan.forwards.length} 件の転送ファイル`,
  );
  const invalid = $derived(days === null);
  const summary = $derived(
    `${many ? `Store ${store} の` : ""}${plan?.tasks.length ?? 0} 件の削除済みタスクと ${plan?.forwards.length ?? 0} 件の転送ファイルを、完全に削除します。`,
  );
</script>

<div class="purge" bind:this={root}>
  <p class="lead">
    削除したタスクのファイルを、コメントと作業ログごと完全に消します。ほかの Store
    へ移したタスクが残した転送ファイルも消します。
  </p>

  <div class="row">
    {#if many}
      <div class="field">
        <label class="lab" for="{uid}-store">Store</label>
        <select
          id="{uid}-store"
          class="ctl"
          value={store}
          disabled={busy}
          onchange={(e) => {
            selected = e.currentTarget.value;
            forget();
          }}
        >
          {#each stores as s (s)}
            <option value={s}>{s}{roOf.current(s) === null ? "" : "（閲覧のみ）"}</option>
          {/each}
        </select>
      </div>
    {/if}
    <div class="field">
      <label class="lab" for="{uid}-days">N 日より古いもの</label>
      <span class="num">
        <input
          id="{uid}-days"
          class="ctl days"
          type="text"
          inputmode="numeric"
          autocomplete="off"
          value={input}
          disabled={busy}
          data-days
          aria-invalid={invalid ? "true" : undefined}
          aria-describedby="{uid}-hint"
          oninput={(e) => {
            input = e.currentTarget.value;
            forget();
          }}
          onkeydown={(e) => {
            if (e.key === "Enter" && !e.isComposing && e.keyCode !== 229)
              void checkNow("[data-days]");
          }}
        >
        <span class="unit">日</span>
      </span>
    </div>
    <Button
      data-check
      disabled={busy || invalid || !store}
      onclick={() => void checkNow("[data-check]")}
    >
      確かめる
    </Button>
  </div>
  <p id="{uid}-hint" class="hint" class:bad={invalid}>
    {invalid
      ? "0 以上の整数で入力してください（0 ですべて、最大 36500）"
      : "0 ですべての削除済みタスクが対象になります。消す前に、対象を確かめます"}
  </p>

  {#if failure}
    <FormError message={failure} />
  {/if}

  <!-- only this short line is announced, not the list -->
  <p class="msg" aria-live="polite">{planLine}</p>

  <div class="plan">
    {#if plan}
      {#if !empty}
        {#if plan.tasks.length > 0}
          <ul class="tasks" aria-label="完全に削除されるタスク">
            {#each plan.tasks as t (t.ulid)}
              <li>
                <IdLabel id={t.id} ulid={t.ulid} {store} />
                <span>{t.title}</span>
              </li>
            {/each}
          </ul>
        {/if}
        <div>
          <Button
            variant="danger"
            data-delete
            disabled={busy || off !== null}
            why={off ?? undefined}
            onclick={() => (dialogOpen = true)}
          >
            完全に削除…
          </Button>
        </div>
      {/if}
    {/if}
  </div>
</div>

<Dialog open={dialogOpen} title="完全に削除する" width={560} {cli} onclose={closeDialog}>
  <p class="p">{summary}</p>
  <p class="p strong">取り消せません。</p>
  {#snippet footer()}
    <Button onclick={closeDialog} disabled={purging}>キャンセル</Button>
    <Button variant="danger" onclick={() => void purge()} disabled={purging}>完全に削除する</Button>
  {/snippet}
</Dialog>

<style>
  .purge {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-8);
  }
  .lead,
  .hint,
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.6;
  }
  .hint.bad {
    color: var(--danger);
  }
  .plan {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-8);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-12);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .lab {
    color: var(--label);
    font-size: var(--fs-12);
  }
  .num {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
  }
  .ctl {
    height: var(--control-h);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    color: var(--ink);
    font-size: var(--fs-13);
  }
  .ctl[aria-invalid="true"] {
    border-color: var(--danger);
  }
  .days {
    width: 6rem;
  }
  .unit {
    font-size: var(--fs-13);
  }
  .tasks {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .tasks li {
    display: flex;
    align-items: baseline;
    gap: var(--space-8);
    font-size: var(--fs-13);
  }
  .p {
    margin: 0 0 var(--space-8);
    font-size: var(--fs-13);
    line-height: 1.7;
  }
  .strong {
    color: var(--danger-ink);
    font-weight: 600;
  }
</style>
