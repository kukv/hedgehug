<script lang="ts">
  import { type ShowView, unwrap } from "../../core/api/client";
  import { defaultStatusIn } from "../../core/schema";
  import { core } from "../context";
  import { useRules } from "../github.svelte";
  import { requiredFields, requiredGap, requiredLabel } from "../kanban/drop";
  import RequiredDialog from "../kanban/RequiredDialog.svelte";
  import { useSubscribe } from "../svelte-store";
  import { patchTask } from "../tree/actions";
  import Button from "../ui/Button.svelte";

  // What to do with a rule that was refused: fill what its status needs and move the task, or leave
  // it. The button needs the rule to tell the status, and the task to tell what is missing: without
  // them, and when nothing is missing, there is only 「このままにする」.
  type Props = {
    task: string; // ULID
    store: string;
    rule: string;
    disabled: boolean;
    off?: string; // why the Store cannot be written
    onread: () => void;
  };
  let { task, store, rule, disabled, off, onread }: Props = $props();

  const c = core();
  const rules = useRules(() => store);
  const schema = useSubscribe(c.schema, () => c.schema.get(store));
  $effect(() => void c.schema.load(store));

  let show = $state.raw<ShowView>();
  let filling = $state(false);
  $effect(() => {
    let live = true;
    void unwrap(c.api.GET("/api/tasks/{id}", { params: { path: { id: task }, query: { store } } }))
      .then((s) => {
        if (live) show = s;
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  });

  const target = $derived.by(() => {
    const r = rules.current?.find((x) => x.id === rule);
    const sc = schema.current;
    if (!r || !sc) return "";
    return r.status ?? defaultStatusIn(sc, r.category ?? "");
  });
  const gap = $derived.by(() => {
    const sc = schema.current;
    if (!show || !sc || target === "" || show.status === target) return null;
    const g = requiredGap(show, requiredFields(sc, show.scope, target), sc, show);
    return g.fill.length + g.elsewhere.length > 0 ? g : null;
  });
  const label = $derived.by(() => {
    const sc = schema.current;
    if (!sc || !gap) return "";
    const names = [...gap.fill, ...gap.elsewhere].map((n) => requiredLabel(sc, n)).join("・");
    const status = sc.statuses?.find((s) => s.name === target)?.label || target;
    return `${names} を入力して${status}にする`;
  });
</script>

{#if gap}
  <Button size="sm" variant="confirm" {disabled} why={off} onclick={() => (filling = true)}>
    {label}
  </Button>
{/if}
<Button size="sm" {disabled} onclick={onread}>このままにする</Button>

{#if filling && gap && show && schema.current}
  {@const task = show}
  <RequiredDialog
    {task}
    status={target}
    fields={gap.fill}
    elsewhere={gap.elsewhere}
    schema={schema.current}
    onsubmit={async (body) => {
      await patchTask(c, task, body);
      onread();
    }}
    onclose={() => (filling = false)}
  />
{/if}
