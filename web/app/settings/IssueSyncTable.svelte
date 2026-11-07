<script lang="ts">
  import { ISSUE_FIELDS, withField } from "../../core/automations";
  import { cliAutomationFields } from "../../core/cli";
  import { type IssueFields, SYNC_MODES } from "../../core/github";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useManyStores } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import CliLine from "../ui/CliLine.svelte";

  // issue_fields: the mode of a new link and what each mode syncs. Each change writes at once. The
  // milestone needs the property it goes to, so its boxes wait for the key.
  type Props = { store: string; fields: IssueFields; off: string | null };
  let { store, fields, off }: Props = $props();

  const c = core();
  const many = useManyStores(c.stores);
  let busy = $state(false);
  // Counts the writes of the default mode that failed: the radios are made again from what is held.
  let refused = $state(0);

  const MODES = ["mirror", "push", "link"].flatMap((v) => SYNC_MODES.filter((m) => m.value === v));
  const key = $derived(fields.props.milestone ?? "");
  const cli = $derived(
    cliAutomationFields({
      defaultSync: fields.default_sync,
      push: fields.push,
      mirror: fields.mirror,
      prop: key ? `milestone=${key}` : undefined,
      store: many.current ? store : undefined,
    }),
  );

  type Change = {
    default_sync?: string;
    push?: string[];
    mirror?: string[];
    props?: Record<string, string>;
  };

  async function write(body: Change, undo: () => void) {
    busy = true;
    try {
      await c.api.updateAutomationFields(body, { store });
    } catch (e) {
      undo();
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }

  const toggle = (mode: "push" | "mirror", name: string, el: HTMLInputElement) => {
    const on = el.checked;
    void write({ [mode]: withField(fields[mode], name, on) }, () => {
      el.checked = !on;
    });
  };

  function setKey(el: HTMLInputElement) {
    const value = el.value.trim();
    if (value === key) return;
    const undo = () => {
      el.value = key;
    };
    const props = { milestone: value };
    // Without a key the milestone cannot sync, so it comes off the lists with it.
    if (value === "") {
      void write(
        {
          props,
          push: withField(fields.push, "milestone", false),
          mirror: withField(fields.mirror, "milestone", false),
        },
        undo,
      );
    } else {
      void write({ props }, undo);
    }
  }

  const locked = (name: string) => busy || off !== null || (name === "milestone" && key === "");
</script>

<div class="modes" role="radiogroup" aria-label="新しくリンクするときの既定">
  <span class="cap">新しくリンクするときの既定</span>
  {#key refused}
    {#each MODES as m (m.value)}
      <label class="mode" class:on={fields.default_sync === m.value}>
        <input
          type="radio"
          name="default-sync-{store}"
          value={m.value}
          checked={fields.default_sync === m.value}
          disabled={busy || off !== null}
          onchange={() => void write({ default_sync: m.value }, () => refused++)}
        >
        {m.label}
      </label>
    {/each}
  {/key}
  <span class="hint">PR は常に GitHub が正本です</span>
</div>

<div class="wrap">
  <table>
    <thead>
      <tr>
        <th scope="col">GitHub のフィールド</th>
        <th scope="col">push モードで同期</th>
        <th scope="col">mirror モードで同期</th>
        <th scope="col">タスク側</th>
        <th scope="col">補足</th>
      </tr>
    </thead>
    <tbody>
      {#each ISSUE_FIELDS as f (f.name)}
        <tr>
          <th scope="row">{f.label}</th>
          {#each ["push", "mirror"] as const as mode (mode)}
            <td>
              <label class="box">
                <input
                  type="checkbox"
                  aria-label="{mode}: {f.label}"
                  checked={fields[mode].includes(f.name)}
                  disabled={locked(f.name)}
                  onchange={(e) => toggle(mode, f.name, e.currentTarget)}
                >
                {fields[mode].includes(f.name) ? "同期する" : "同期しない"}
              </label>
            </td>
          {/each}
          <td class="to">
            {#if f.name === "milestone"}
              props.<input
                aria-label="マイルストーンのプロパティ"
                placeholder="target"
                autocomplete="off"
                value={key}
                disabled={busy || off !== null}
                onchange={(e) => setKey(e.currentTarget)}
              >
            {:else}
              {f.to}
            {/if}
          </td>
          <td class="note">{f.note}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
<p class="hint">
  push モードで GitHub 側が手で編集されていたら、次の push で上書きし、上書きしたことを通知
  に出します。リンクごとに設定を上書きできます。
</p>
<CliLine command={cli} />

<style>
  .modes {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-8);
  }
  .cap {
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .mode {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    min-height: var(--control-h);
    padding: 0 var(--space-10);
    border-radius: var(--radius-6);
    background: var(--sunken);
    color: var(--ink-2);
  }
  .mode.on {
    background: var(--accent-weak);
    color: var(--accent-ink);
    font-weight: 600;
  }
  .hint {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .wrap {
    overflow-x: auto;
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
  }
  table {
    width: 100%;
    min-width: 860px;
    border-collapse: collapse;
  }
  th,
  td {
    padding: var(--space-8);
    border-bottom: 1px solid var(--line);
    text-align: left;
    font-size: var(--fs-12);
  }
  thead th {
    color: var(--muted);
    font-weight: 500;
  }
  tbody th {
    font-size: var(--fs-13);
    font-weight: 500;
  }
  tbody tr:last-child > * {
    border-bottom: 0;
  }
  .box {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    color: var(--ink-2);
  }
  .to {
    font-family: var(--font-mono);
  }
  .to input {
    width: 7rem;
    padding: 2px var(--space-6);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
  }
  .note {
    color: var(--ink-2);
  }
</style>
