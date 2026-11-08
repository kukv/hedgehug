<script lang="ts">
  import { CATEGORY_LABEL, type Category } from "../../core/categories";
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import Button from "../ui/Button.svelte";
  import SchemaRowActions from "./SchemaRowActions.svelte";
  import type { SchemaAction } from "./schema-action";
  import { count } from "./schema-text";

  type Props = {
    store: string;
    schema: SchemaView;
    usage: SchemaUsage | null;
    off: string | null;
    onaction: (a: SchemaAction) => void;
  };
  let { schema, usage, off, onaction }: Props = $props();

  const uid = $props.id();
  const statuses = $derived(schema.statuses ?? []);
</script>

<section class="sec" aria-labelledby="{uid}-statuses">
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
      追加…
    </Button>
  </div>
  <table aria-labelledby="{uid}-statuses">
    <thead>
      <tr>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">カテゴリ</th>
        <th scope="col">既定</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each statuses as s (s.name)}
        <tr>
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
                names={statuses.map((x) => x.name)}
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
  .name {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
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
