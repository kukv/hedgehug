<script lang="ts">
  import "./schema-table.css";
  import { CATEGORY_LABEL, type Category } from "../../core/categories";
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import Button from "../ui/Button.svelte";
  import { useOrder } from "./order.svelte";
  import { sortableRow } from "./reorder";
  import SchemaHandle from "./SchemaHandle.svelte";
  import SchemaRowActions from "./SchemaRowActions.svelte";
  import type { SchemaAction } from "./schema-action";
  import { count, moveKeys } from "./schema-text";

  type Props = {
    store: string;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    onaction: (a: SchemaAction) => void;
  };
  let { store, schema, usage, off, onaction }: Props = $props();

  const uid = $props.id();
  const statuses = $derived(schema.statuses ?? []);
  const order = useOrder(
    () => store,
    () => off,
  );
  const names = $derived(statuses.map((x) => x.name));
  const move = (name: string, from: number, to: number) =>
    order.move(`statuses/${name}`, names, from, to, (api, s, next) =>
      api.orderStatuses({ names: next }, { store: s }),
    );
</script>

<section class="schema-sec" aria-labelledby="{uid}-statuses">
  <h2 id="{uid}-statuses">ステータス</h2>
  <p class="note">
    どれも未着手・進行中・完了・中止のいずれかのカテゴリに属し、各カテゴリに既定が 1 つあります。
  </p>
  <div>
    <Button
      size="sm"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => onaction({ kind: "add", target: "status" })}
    >
      追加
    </Button>
  </div>
  <span id="{uid}-move" class="sr">{moveKeys()} で移動</span>
  <table aria-labelledby="{uid}-statuses">
    <thead>
      <tr>
        <th scope="col"><span class="sr">並べ替え</span></th>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">カテゴリ</th>
        <th scope="col">既定</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each statuses as s, i (s.name)}
        <tr
          use:sortableRow={{
            list: "statuses",
            name: s.name,
            index: i,
            disabled: off !== null,
            onmove: (a, b) => void move(s.name, a, b),
          }}
        >
          <td>
            <SchemaHandle
              id="statuses/{s.name}"
              subject={s.name}
              hint="{uid}-move"
              disabled={order.busy}
              {off}
              onstep={(by) => void move(s.name, i, i + by)}
            />
          </td>
          <th scope="row">
            <span class="name"
              ><StatusIcon status={s.name} category={s.category} color={s.color} size={14} />
              {s.name}</span
            >
          </th>
          <td>{s.label ?? ""}</td>
          <td>{CATEGORY_LABEL[s.category as Category] ?? s.category}</td>
          <td>{s.default ? "既定" : ""}</td>
          <td class="num">{count(usage, usage?.statuses[s.name])}</td>
          <td>
            <div class="acts">
              <SchemaRowActions
                target="status"
                old={s.name}
                {names}
                edit="status"
                used={usage?.statuses[s.name]}
                isDefault={s.default === true}
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
  .name {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
  }
</style>
