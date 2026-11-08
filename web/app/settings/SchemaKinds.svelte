<script lang="ts">
  import "./schema-table.css";
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import KindIcon from "../icons/KindIcon.svelte";
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
  const kinds = $derived(schema.kinds ?? []);
  const order = useOrder(
    () => store,
    () => off,
  );
  const names = $derived(kinds.map((x) => x.name));
  const move = (name: string, from: number, to: number) =>
    order.move(`kinds/${name}`, names, from, to, (api, s, next) =>
      api.orderKinds({ names: next }, { store: s }),
    );

  const ROLE: Record<string, string> = { container: "グループ", work: "アイテム" };
  const parentsText = (parents: string[] | null): string =>
    parents === null
      ? "どこにでも"
      : parents.length === 0
        ? "置けない"
        : parents.map((p) => (p === "root" ? "最上位" : p)).join("、");
</script>

<section class="schema-sec" aria-labelledby="{uid}-kinds">
  <h2 id="{uid}-kinds">種類</h2>
  <p class="note">
    役割はグループ（ほかの種類をまとめる単位）とアイテム（作業の項目）の 2
    つです。どの下に置けるかに反する配置は書き込み時に拒否されます。
  </p>
  <div>
    <Button
      size="sm"
      disabled={off !== null}
      why={off ?? undefined}
      onclick={() => onaction({ kind: "add", target: "kind" })}
    >
      追加
    </Button>
  </div>
  <span id="{uid}-move" class="sr">{moveKeys()} で移動</span>
  <table aria-labelledby="{uid}-kinds">
    <thead>
      <tr>
        <th scope="col"><span class="sr">並べ替え</span></th>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">役割</th>
        <th scope="col">どの下に置けるか</th>
        <th scope="col">既定</th>
        <th scope="col">既定のテンプレート</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each kinds as k, i (k.name)}
        <tr
          use:sortableRow={{
            list: "kinds",
            name: k.name,
            index: i,
            disabled: off !== null,
            onmove: (a, b) => void move(k.name, a, b),
          }}
        >
          <td>
            <SchemaHandle
              id="kinds/{k.name}"
              subject={k.name}
              hint="{uid}-move"
              disabled={order.busy}
              {off}
              onstep={(by) => void move(k.name, i, i + by)}
            />
          </td>
          <th scope="row">
            <span class="name"
              ><KindIcon kind={k.name} role={k.role} icon={k.icon} color={k.color} size={14} />
              {k.name}</span
            >
          </th>
          <td>{k.label ?? ""}</td>
          <td>{ROLE[k.role] ?? k.role}</td>
          <td>{parentsText(k.parents)}</td>
          <td>{k.default ? "既定" : ""}</td>
          <td>{k.template ?? ""}</td>
          <td class="num">{count(usage, usage?.kinds[k.name])}</td>
          <td>
            <div class="acts">
              <SchemaRowActions
                target="kind"
                old={k.name}
                {names}
                edit="kind"
                used={usage?.kinds[k.name]}
                isDefault={k.default === true}
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
