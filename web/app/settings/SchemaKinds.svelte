<script lang="ts">
  import type { SchemaView } from "../../core/schema";
  import type { SchemaUsage } from "../../core/schema-usage";
  import KindIcon from "../icons/KindIcon.svelte";
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
  const kinds = $derived(schema.kinds ?? []);

  const parentsText = (parents: string[] | null): string =>
    parents === null
      ? "どこにでも"
      : parents.length === 0
        ? "置けない"
        : parents.map((p) => (p === "root" ? "ルート" : p)).join("、");
</script>

<section class="sec" aria-labelledby="{uid}-kinds">
  <h2 id="{uid}-kinds">種類</h2>
  <p class="note">
    役割は container（まとめる単位）と work（作業項目）の 2
    つです。置ける親に反する配置は書き込み時に拒否されます。
  </p>
  <table aria-labelledby="{uid}-kinds">
    <thead>
      <tr>
        <th scope="col">名前</th>
        <th scope="col">表示名</th>
        <th scope="col">役割</th>
        <th scope="col">置ける親</th>
        <th scope="col">既定</th>
        <th scope="col">既定のテンプレート</th>
        <th scope="col" class="num">タスク</th>
        <th scope="col"><span class="sr">操作</span></th>
      </tr>
    </thead>
    <tbody>
      {#each kinds as k (k.name)}
        <tr>
          <th scope="row">
            <span class="name"
              ><KindIcon kind={k.name} role={k.role} icon={k.icon} size={14} />{k.name}</span
            >
          </th>
          <td>{k.label ?? ""}</td>
          <td>{k.role}</td>
          <td>{parentsText(k.parents)}</td>
          <td>{k.default ? "既定" : ""}</td>
          <td>{k.template ?? ""}</td>
          <td class="num">{count(usage, usage?.kinds[k.name])}</td>
          <td>
            <div class="acts">
              <SchemaRowActions
                target="kind"
                old={k.name}
                names={kinds.map((x) => x.name)}
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
