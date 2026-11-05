<script lang="ts">
  import type { Schemas } from "../../core/api/client";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { useDisplayId } from "../svelte-store";
  import MoveScopedChoice from "./MoveScopedChoice.svelte";
  import { unlinkedRows } from "./move-plan";

  // What the plan says will change: the ID, the parent, the references it cuts, the choice of the
  // scoped schema, and who loses sight of the task.
  let {
    plan,
    names,
    scoped,
    onscoped,
  }: {
    plan: Schemas["MovePlanView"];
    names: { id: string; parent: string; hidden: string[] };
    scoped: "carry" | "drop" | undefined;
    onscoped: (v: "carry" | "drop") => void;
  } = $props();
  const uid = $props.id();
  const did = useDisplayId(core().stores);
</script>

<section class="part" aria-labelledby="{uid}-changes">
  <h3 id="{uid}-changes">移動先で変わること</h3>
  <dl>
    <dt>ID</dt>
    <dd>{names.id}</dd>
    <dt>親</dt>
    <dd>{names.parent}</dd>
    <dt>移動元</dt>
    <dd>転送ファイルを残し、古い ID からも辿れます</dd>
  </dl>
</section>

{#if plan.unlinked.length > 0}
  <section class="part" aria-labelledby="{uid}-cut">
    <h3 id="{uid}-cut">外す参照 <span class="note">— 参照は Store の中に閉じるため</span></h3>
    <ul class="refs">
      {#each unlinkedRows(plan.unlinked) as r (r.key)}
        <li>
          <span class="kind">{r.kind}</span>
          <span class="mono">{r.target}</span>
          <span class="mono on-task">{did.current(plan.from, r.id)}</span>
        </li>
      {/each}
    </ul>
  </section>
{/if}

{#if plan.scoped}
  <MoveScopedChoice dropped={plan.scoped.dropped} {scoped} {onscoped} />
{/if}

{#if plan.hidden.length > 0}
  <div class="warn" role="note">
    <Icon name="warning" size={16} />
    <div>
      <b>見える範囲が狭くなります</b>
      {#each names.hidden as line (line)}
        <p>{line}</p>
      {/each}
    </div>
  </div>
{/if}

<style>
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  .note {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 400;
  }
  .part {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding-top: var(--space-12);
    border-top: 1px solid var(--line);
  }
  h3 {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  dl {
    display: grid;
    grid-template-columns: 110px minmax(0, 1fr);
    gap: var(--space-8) var(--space-12);
    margin: 0;
    line-height: 1.55;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
  }
  .refs {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .refs li {
    display: flex;
    gap: var(--space-10);
    align-items: center;
    min-height: 34px;
    padding: 0 var(--space-10);
    border-radius: var(--radius-6);
    background: var(--sunken);
  }
  .kind {
    min-width: 130px;
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .on-task {
    color: var(--muted);
  }
  .warn {
    display: flex;
    gap: var(--space-10);
    padding: var(--space-12) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--warning-weak);
    color: var(--warning);
    line-height: 1.6;
  }
  .warn p {
    margin: var(--space-6) 0 0;
  }
</style>
