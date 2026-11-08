<script lang="ts">
  import "./schema-table.css";
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import Button from "../ui/Button.svelte";
  import { useOrder } from "./order.svelte";
  import { sortableRow } from "./reorder";
  import SchemaHandle from "./SchemaHandle.svelte";
  import SchemaRowActions from "./SchemaRowActions.svelte";
  import type { SchemaAction } from "./schema-action";
  import { count, moveKeys, propEntries, propNote } from "./schema-text";

  type Props = {
    store: string;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    onaction: (a: SchemaAction) => void;
  };
  let { store, schema, usage, off, onaction }: Props = $props();

  const uid = $props.id();
  const entries = $derived(propEntries(schema));
  const order = useOrder(
    () => store,
    () => off,
  );
  const names = $derived(entries.map(([n]) => n));
  const move = (name: string, from: number, to: number) =>
    order.move(`props/${name}`, names, from, to, (api, s, next) =>
      api.orderProps({ names: next }, { store: s }),
    );
  const moveValue = (prop: string, values: string[], v: string, from: number, to: number) =>
    order.move(`values/${prop}/${v}`, values, from, to, (api, s, next) =>
      api.orderValues(prop, { names: next }, { store: s }),
    );
</script>

<section class="schema-sec" aria-labelledby="{uid}-props">
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
  <span id="{uid}-move" class="sr">{moveKeys()} で移動</span>
  <table aria-labelledby="{uid}-props">
    <thead>
      <tr>
        <th scope="col"><span class="sr">並べ替え</span></th>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">型</th>
        <th scope="col">設定</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each entries as [name, p], i (name)}
        <tr
          use:sortableRow={{
            list: "props",
            name,
            index: i,
            disabled: off !== null,
            onmove: (a, b) => void move(name, a, b),
          }}
        >
          <td>
            <SchemaHandle
              id="props/{name}"
              subject={name}
              hint="{uid}-move"
              disabled={order.busy}
              {off}
              onstep={(by) => void move(name, i, i + by)}
            />
          </td>
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
                {names}
                edit="prop"
                used={usage?.props[name]}
                addValue={p.type === "enum" || p.type === "multi_enum"}
                {off}
                {onaction}
              />
            </div>
          </td>
        </tr>
        {#each p.values ?? [] as v, vi (v)}
          <tr
            class="value"
            use:sortableRow={{
              list: `values:${name}`,
              name: v,
              index: vi,
              disabled: off !== null,
              onmove: (a, b) => void moveValue(name, p.values ?? [], v, a, b),
            }}
          >
            <td>
              <SchemaHandle
                id="values/{name}/{v}"
                subject="{name} の {v}"
                hint="{uid}-move"
                disabled={order.busy}
                {off}
                onstep={(by) => void moveValue(name, p.values ?? [], v, vi, vi + by)}
              />
            </td>
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
          <td colspan="7" class="none">プロパティはありません</td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<style>
  .value th {
    padding-left: var(--space-24);
    font-weight: 400;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .none {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
</style>
