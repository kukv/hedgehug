<script lang="ts">
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import type { SchemaAction } from "./schema-action";

  type Props = {
    store: string;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    onaction: (a: SchemaAction) => void;
  };
  let { schema }: Props = $props();

  const uid = $props.id();
  const transitions = $derived(Object.entries(schema.workflow?.transitions ?? {}));
  const required = $derived(Object.entries(schema.workflow?.required ?? {}));

  const nameList = (names: readonly string[] | null | undefined): string =>
    (names ?? []).length === 0 ? "なし" : (names ?? []).join("、");
</script>

<section class="sec" aria-labelledby="{uid}-workflow">
  <h2 id="{uid}-workflow">ワークフロー</h2>
  <p class="note">
    制約は、付けたステータスにだけ適用されます。付けていないステータスはどこへでも移れます。
  </p>
  <h3>遷移の制約</h3>
  {#if transitions.length === 0}
    <p class="none">制約はありません</p>
  {:else}
    <ul class="flow">
      {#each transitions as [from, to] (from)}
        <li><span class="mono">{from}</span> から移せるのは {nameList(to)} だけ</li>
      {/each}
    </ul>
  {/if}
  <h3>入るときに必須の項目</h3>
  {#if required.length === 0}
    <p class="none">必須の項目はありません</p>
  {:else}
    <ul class="flow">
      {#each required as [status, fields] (status)}
        <li><span class="mono">{status}</span> に入るには {nameList(fields)} が必要</li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  h2 {
    margin: 0;
    font-size: var(--fs-15);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-13);
    font-weight: 600;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .none {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .flow {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--fs-13);
  }
</style>
