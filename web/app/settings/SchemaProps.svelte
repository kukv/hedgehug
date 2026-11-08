<script lang="ts">
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import Button from "../ui/Button.svelte";
  import SchemaRowActions from "./SchemaRowActions.svelte";
  import type { SchemaAction } from "./schema-action";
  import { count, propEntries, propNote } from "./schema-text";

  type Props = {
    store: string;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    onaction: (a: SchemaAction) => void;
  };
  let { schema, usage, off, onaction }: Props = $props();

  const uid = $props.id();
  const entries = $derived(propEntries(schema));
</script>

<section class="sec" aria-labelledby="{uid}-props">
  <h2 id="{uid}-props">プロパティ</h2>
  <p class="note">値の件数は、その値を持つタスクの数です。</p>
  <div>
    <Button
      size="sm"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => onaction({ kind: "add", target: "prop" })}
    >
      追加…
    </Button>
  </div>
  <table aria-labelledby="{uid}-props">
    <thead>
      <tr>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">型</th>
        <th scope="col">設定</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each entries as [name, p] (name)}
        <tr>
          <th scope="row">{name}</th>
          <td>{p.label ?? ""}</td>
          <td class="mono">{p.type}</td>
          <td>{propNote(p)}</td>
          <td class="num">{count(usage, usage?.props[name])}</td>
          <td>
            <div class="acts">
              <SchemaRowActions
                target="prop"
                old={name}
                names={entries.map(([n]) => n)}
                edit="prop"
                used={usage?.props[name]}
                addValue={p.type === "enum" || p.type === "multi_enum"}
                {off}
                {onaction}
              />
            </div>
          </td>
        </tr>
        {#each p.values ?? [] as v (v)}
          <tr class="value">
            <th scope="row"><span class="vname">{name} › {v}</span></th>
            <td colspan="3"></td>
            <td class="num">{count(usage, usage?.values[name]?.[v])}</td>
            <td>
              <div class="acts">
                <SchemaRowActions
                  target="value"
                  old={v}
                  names={p.values ?? []}
                  prop={name}
                  used={usage?.values[name]?.[v]}
                  {off}
                  {onaction}
                />
              </div>
            </td>
          </tr>
        {/each}
      {:else}
        <tr>
          <td colspan="6" class="none">プロパティはありません</td>
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
  .value th {
    padding-left: var(--space-24);
    font-weight: 400;
  }
  .num {
    text-align: right;
  }
  .acts {
    display: flex;
    gap: var(--space-6);
    justify-content: flex-end;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .none {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
