<script lang="ts">
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import Button from "../ui/Button.svelte";
  import SchemaRowActions from "./SchemaRowActions.svelte";
  import type { SchemaAction } from "./schema-action";
  import { count, propNote, relationEntries } from "./schema-text";

  // blocked_by is a relation built in: it is not in the schema, and its name cannot be changed. Its
  // number is how many tasks have one (undefined until the tasks are read).
  type Props = {
    store: string;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    onaction: (a: SchemaAction) => void;
    blocked: number | undefined;
  };
  let { schema, usage, off, onaction, blocked }: Props = $props();

  const uid = $props.id();
  const entries = $derived(relationEntries(schema));
</script>

<section class="sec" aria-labelledby="{uid}-relations">
  <h2 id="{uid}-relations">関連</h2>
  <p class="note">
    関連は同じ Store のタスクだけを参照できます。blocked_by
    は組み込みの関連のため、名前の変更とマージはできません。
  </p>
  <div>
    <Button
      size="sm"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => onaction({ kind: "add", target: "relation" })}
    >
      追加…
    </Button>
  </div>
  <table aria-labelledby="{uid}-relations">
    <thead>
      <tr>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">設定</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <th scope="row">blocked_by</th>
        <td>ブロックされている</td>
        <td>複数 · 組み込み</td>
        <td class="num">{count(usage, blocked)}</td>
        <td></td>
      </tr>
      {#each entries as [name, p] (name)}
        <tr>
          <th scope="row">{name}</th>
          <td>{p.label ?? ""}</td>
          <td>{propNote(p)}</td>
          <td class="num">{count(usage, usage?.props[name])}</td>
          <td>
            <div class="acts">
              <SchemaRowActions
                target="prop"
                old={name}
                names={entries.map(([n]) => n)}
                edit="relation"
                used={usage?.props[name]}
                {off}
                {onaction}
              />
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<style>
  h2 {
    margin: 0;
    font-size: var(--fs-15);
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
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th,
  td {
    padding: var(--space-8) var(--space-12);
    border-bottom: 1px solid var(--line);
    text-align: left;
  }
  thead th {
    color: var(--muted);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  tbody th {
    font-weight: 600;
  }
  .num {
    text-align: right;
  }
  .acts {
    display: flex;
    gap: var(--space-6);
    justify-content: flex-end;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
