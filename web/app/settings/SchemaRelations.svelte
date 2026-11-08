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
  import { count, moveKeys, propNote, relationEntries } from "./schema-text";

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
  let { store, schema, usage, off, onaction, blocked }: Props = $props();

  const uid = $props.id();
  const entries = $derived(relationEntries(schema));
  const order = useOrder(
    () => store,
    () => off,
  );
  const names = $derived(entries.map(([n]) => n));
  const move = (name: string, from: number, to: number) =>
    order.move(`relations/${name}`, names, from, to, (api, s, next) =>
      api.orderProps({ names: next }, { store: s }),
    );
</script>

<section class="schema-sec" aria-labelledby="{uid}-relations">
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
  <span id="{uid}-move" class="sr">{moveKeys()} で移動</span>
  <table aria-labelledby="{uid}-relations">
    <thead>
      <tr>
        <th scope="col"><span class="sr">並べ替え</span></th>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">設定</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td></td>
        <th scope="row">blocked_by</th>
        <td>ブロックされている</td>
        <td>複数 · 組み込み</td>
        <td class="num">{count(usage, blocked)}</td>
        <td></td>
      </tr>
      {#each entries as [name, p], i (name)}
        <tr
          use:sortableRow={{
            list: "relations",
            name,
            index: i,
            disabled: off !== null,
            onmove: (a, b) => void move(name, a, b),
          }}
        >
          <td>
            <SchemaHandle
              id="relations/{name}"
              subject={name}
              hint="{uid}-move"
              disabled={order.busy}
              {off}
              onstep={(by) => void move(name, i, i + by)}
            />
          </td>
          <th scope="row">{name}</th>
          <td>{p.label ?? ""}</td>
          <td>{propNote(p)}</td>
          <td class="num">{count(usage, usage?.props[name])}</td>
          <td>
            <div class="acts">
              <SchemaRowActions
                target="prop"
                old={name}
                {names}
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
</style>
