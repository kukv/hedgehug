<script lang="ts">
  import { cliTemplateEdit, cliTemplateRm } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import type { Template } from "../../core/planning";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";
  import FormError from "../ui/FormError.svelte";
  import KindTemplateDefaults from "./KindTemplateDefaults.svelte";

  // The templates of a Store: the file as it is, checked by the server when it is saved.
  type Props = { store: string; off: string | null };
  let { store, off }: Props = $props();

  const { api, schema } = core();
  const sc = useSubscribe(schema, () => schema.get(store));
  const defaults = $derived(sc.current?.templates?.default ?? {});
  let defaultsFailure = $state<string | null>(null);
  $effect(() => {
    void schema.load(store).catch(() => undefined);
  });

  // The default template of a kind: all the defaults go back, with the one of this kind changed.
  async function setDefault(kind: string, name: string) {
    defaultsFailure = null;
    const { [kind]: _, ...rest } = defaults;
    try {
      await api.setTemplateDefaults(
        { default: name === "" ? rest : { ...rest, [kind]: name } },
        { store },
      );
      await schema.reload(store);
    } catch (e) {
      defaultsFailure = errorMessage(e);
    }
  }
  const uid = $props.id();

  let list = $state.raw<Template[]>([]);
  let rev = $state(0);
  let listError = $state<string | null>(null);
  $effect(() => {
    const s = store;
    void rev;
    let live = true;
    api
      .listTemplates({ store: s })
      .then((r) => {
        if (!live) return;
        list = r ?? [];
        listError = null;
      })
      .catch((e: unknown) => {
        if (live) listError = errorMessage(e);
      });
    return () => {
      live = false;
    };
  });

  let open = $state<string | null>(null);
  let content = $state("");
  let saved = $state("");
  let exists = $state(false);
  let adding = $state(false);
  let draftName = $state("");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  async function show(name: string) {
    failure = null;
    try {
      const f = await api.getTemplate(name, { store });
      open = f.name;
      content = f.content;
      saved = f.exists ? f.content : "";
      exists = f.exists;
      adding = false;
    } catch (e) {
      failure = errorMessage(e);
    }
  }

  async function save() {
    if (!open || busy) return;
    busy = true;
    failure = null;
    try {
      await api.putTemplate(open, { content }, { store });
      saved = content;
      exists = true;
      rev++;
      toast({ message: `テンプレート ${open} を保存しました` });
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function remove() {
    if (!open || busy) return;
    busy = true;
    failure = null;
    try {
      await api.removeTemplate(open, { store });
      toast({ message: `テンプレート ${open} を削除しました` });
      open = null;
      rev++;
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<section class="sec" aria-labelledby="{uid}-h">
  <h2 id="{uid}-h">テンプレート</h2>
  {#if listError}
    <FormError message={listError} />
  {/if}
  <div class="cols">
    <ul aria-label="テンプレートの一覧">
      {#each list as t (t.name)}
        <li>
          <button
            type="button"
            aria-current={open === t.name ? "true" : undefined}
            onclick={() => void show(t.name)}
          >
            <span>{t.name}</span>
            <span class="meta"
              >{t.kind ?? ""}{t.children > 0 ? ` · 子タスク ${t.children}` : ""}</span
            >
          </button>
        </li>
      {:else}
        <li class="none">テンプレートはありません</li>
      {/each}
      <li>
        {#if adding}
          <form
            onsubmit={(e) => {
              e.preventDefault();
              if (draftName.trim() !== "") void show(draftName.trim());
            }}
          >
            <input
              type="text"
              aria-label="テンプレートの名前"
              autocomplete="off"
              value={draftName}
              oninput={(e) => (draftName = e.currentTarget.value)}
            >
            <Button type="submit" size="sm" disabled={draftName.trim() === ""}>開く</Button>
          </form>
        {:else}
          <Button
            size="sm"
            disabled={off !== null}
            why={off ?? undefined}
            onclick={() => {
              adding = true;
              draftName = "";
            }}
          >
            ＋ テンプレート
          </Button>
        {/if}
      </li>
    </ul>
    {#if open}
      <div class="edit">
        <span class="path">templates/{open}.md</span>
        <textarea
          aria-label="テンプレートのファイル"
          rows="16"
          readonly={off !== null}
          value={content}
          oninput={(e) => (content = e.currentTarget.value)}
        ></textarea>
        {#if failure}
          <FormError message={failure} />
        {/if}
        <div class="acts">
          <Button
            variant="primary"
            disabled={busy || off !== null || (exists && content === saved)}
            why={off ?? undefined}
            onclick={() => void save()}
          >
            保存
          </Button>
          {#if exists}
            <Button
              variant="danger"
              disabled={busy || off !== null}
              why={off ?? undefined}
              onclick={() => void remove()}
            >
              削除
            </Button>
          {/if}
        </div>
        <CliLine command={exists ? cliTemplateRm(open, store) : cliTemplateEdit(open, store)} />
      </div>
    {:else if failure}
      <FormError message={failure} />
    {/if}
  </div>
</section>

<section class="sec" aria-labelledby="{uid}-d">
  <h2 id="{uid}-d">種類ごとの既定のテンプレート</h2>
  <p class="none">種類を指定せずに選ぶと使う、種類ごとのテンプレートです</p>
  {#if defaultsFailure}
    <FormError message={defaultsFailure} />
  {/if}
  <KindTemplateDefaults
    names={list.map((t) => t.name)}
    values={defaults}
    disabled={off !== null}
    none={() => "なし"}
    onchange={(k, n) => void setDefault(k, n)}
  />
</section>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: var(--space-10);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
    font-weight: 600;
  }
  .cols {
    display: grid;
    grid-template-columns: minmax(220px, 1fr) minmax(0, 3fr);
    gap: var(--space-16);
    align-items: start;
  }
  @media (max-width: 760px) {
    .cols {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  ul {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li button[aria-current="true"] {
    background: var(--accent-weak);
  }
  li > button {
    display: flex;
    flex-direction: column;
    width: 100%;
    padding: var(--space-6) var(--space-8);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    text-align: left;
    cursor: pointer;
  }
  .meta,
  .path,
  .none {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .path {
    font-family: var(--font-mono);
  }
  form {
    display: flex;
    gap: var(--space-6);
  }
  input {
    flex: 1;
    min-width: 0;
    min-height: 30px;
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
  }
  .edit {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    min-width: 0;
  }
  textarea {
    padding: var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    line-height: 1.7;
    resize: vertical;
  }
  .acts {
    display: flex;
    gap: var(--space-8);
  }
</style>
