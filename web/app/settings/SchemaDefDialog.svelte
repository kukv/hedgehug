<script lang="ts" module>
  export type DefTarget = "status" | "kind" | "prop" | "relation";

  const NOUN: Record<DefTarget, string> = {
    status: "ステータス",
    kind: "種類",
    prop: "プロパティ",
    relation: "関連",
  };

  // The types a property can be added as; task_ref is a relation, added from its own tab.
  const PROP_TYPES = [
    "text",
    "number",
    "bool",
    "date",
    "datetime",
    "duration",
    "enum",
    "multi_enum",
    "user",
    "url",
  ] as const;
</script>

<script lang="ts">
  import { tick, untrack } from "svelte";
  import { CATEGORIES, CATEGORY_LABEL, type Category } from "../../core/categories";
  import { errorMessage } from "../../core/messages";
  import type { SchemaView } from "../../core/schema";
  import { core } from "../context";
  import { onLinkClick, router } from "../router";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";
  import FormError from "../ui/FormError.svelte";
  import ColorPicker from "./ColorPicker.svelte";
  import IconPicker from "./IconPicker.svelte";
  import { failureOf } from "./schema-text";

  // Adds a definition of the schema (name given), or edits one (name of the definition given). Only the
  // fields that changed are sent. ulidOf finds the task of an ID that a refusal names.
  type Props = {
    store: string;
    target: DefTarget;
    name?: string;
    schema: SchemaView;
    ulidOf: (id: string) => string | undefined;
    onclose: () => void;
    ondone: () => void;
  };
  let { store, target, name: editing, schema, ulidOf, onclose, ondone }: Props = $props();

  const c = core();
  const uid = $props.id();
  // The dialog is opened for one definition and closed before another is asked for.
  const adding = untrack(() => editing === undefined);
  const noun = untrack(() => NOUN[target]);

  // What the definition has now (the schema may change while the dialog is open: the dialog keeps the
  // fields it was opened with).
  const status = untrack(() => schema.statuses?.find((s) => s.name === editing));
  const kind = untrack(() => schema.kinds?.find((k) => k.name === editing));
  const prop = untrack(() => (editing === undefined ? undefined : schema.props[editing]));
  const kinds = untrack(() => schema.kinds ?? []);

  let name = $state("");
  let label = $state(status?.label ?? kind?.label ?? prop?.label ?? "");
  let category = $state(status?.category ?? "todo");
  let color = $state(status?.color ?? "");
  let isDefault = $state(status?.default ?? kind?.default ?? false);
  let role = $state(kind?.role ?? "work");
  let anywhere = $state(kind ? kind.parents === null : true);
  let parents = $state<string[]>(kind?.parents ?? []);
  let icon = $state(kind?.icon ?? "");
  let template = $state(kind?.template ?? "");
  let propType = $state<string>(prop?.type ?? "text");
  let valuesText = $state("");
  let multi = $state(prop?.multi ?? false);
  let symmetric = $state(prop?.symmetric ?? false);
  let inverse = $state(prop?.inverse ?? "");

  let busy = $state(false);
  let err = $state.raw<unknown>(null);
  let form: HTMLElement | undefined = $state();

  // The templates of the Store, for the kind's default one.
  let templates = $state.raw<string[]>([]);
  let templatesFailure = $state<string | null>(null);
  if (untrack(() => target === "kind")) {
    Promise.resolve()
      .then(() => c.api.listTemplates({ store }))
      .then((r) => (templates = (r ?? []).map((t) => t.name)))
      .catch((e: unknown) => (templatesFailure = errorMessage(e)));
  }
  const templateNames = $derived(
    kind?.template && !templates.includes(kind.template)
      ? [kind.template, ...templates]
      : templates,
  );

  const statusDefaultLocked = status?.default === true;
  const kindDefaultLocked = kind?.default === true;
  const hasValues = $derived(propType === "enum" || propType === "multi_enum");
  const values = $derived([
    ...new Set(
      valuesText
        .split("\n")
        .map((v) => v.trim())
        .filter((v) => v !== ""),
    ),
  ]);
  const kindNames = $derived(["root", ...kinds.map((k) => k.name)]);
  const noParent = $derived(target === "kind" && !anywhere && parents.length === 0);

  function toggleParent(p: string, on: boolean): void {
    parents = on ? [...parents, p] : parents.filter((x) => x !== p);
  }
  const sameSet = (a: string[], b: string[]): boolean =>
    a.length === b.length && a.every((x) => b.includes(x));
  const orderedParents = $derived(kindNames.filter((p) => parents.includes(p)));

  // The body of the write: all the fields for an add, those that changed for an edit.
  const statusBody = $derived.by(() => {
    const l = label.trim();
    if (adding) {
      return {
        name: name.trim(),
        category: category as Category,
        ...(l ? { label: l } : {}),
        ...(color ? { color } : {}),
      };
    }
    return {
      ...(l !== (status?.label ?? "") ? { label: l } : {}),
      ...(!statusDefaultLocked && category !== status?.category
        ? { category: category as Category }
        : {}),
      ...(color !== (status?.color ?? "") ? { color } : {}),
      ...(isDefault && !statusDefaultLocked ? { default: true } : {}),
    };
  });
  const kindBody = $derived.by(() => {
    const l = label.trim();
    const parentsPart = anywhere ? { anywhere: true } : { parents: orderedParents };
    if (adding) {
      return {
        name: name.trim(),
        role: role as "container" | "work",
        ...(l ? { label: l } : {}),
        ...(icon ? { icon } : {}),
        ...(template ? { template } : {}),
        ...parentsPart,
      };
    }
    const parentsChanged = anywhere
      ? kind?.parents !== null
      : kind?.parents === null || !sameSet(orderedParents, kind?.parents ?? []);
    return {
      ...(l !== (kind?.label ?? "") ? { label: l } : {}),
      ...(!kindDefaultLocked && role !== kind?.role ? { role: role as "container" | "work" } : {}),
      ...(parentsChanged ? parentsPart : {}),
      ...(icon !== (kind?.icon ?? "") ? { icon } : {}),
      ...(template !== (kind?.template ?? "") ? { template } : {}),
      ...(isDefault && !kindDefaultLocked && role === "work" ? { default: true } : {}),
    };
  });
  const relationFlags = $derived.by(() => {
    const inv = symmetric ? "" : inverse.trim();
    return {
      ...(multi !== (prop?.multi ?? false) ? { multi } : {}),
      ...(symmetric !== (prop?.symmetric ?? false) ? { symmetric } : {}),
      ...(inv !== (prop?.inverse ?? "") ? { inverse: inv } : {}),
    };
  });
  const propBody = $derived.by(() => {
    const l = label.trim();
    if (adding) {
      const type = target === "relation" ? "task_ref" : propType;
      const inv = symmetric ? "" : inverse.trim();
      return {
        name: name.trim(),
        type: type as (typeof PROP_TYPES)[number] | "task_ref",
        ...(l ? { label: l } : {}),
        ...(hasValues && target === "prop" ? { values } : {}),
        ...(target === "relation"
          ? {
              ...(multi ? { multi } : {}),
              ...(symmetric ? { symmetric } : {}),
              ...(inv ? { inverse: inv } : {}),
            }
          : {}),
      };
    }
    return {
      ...(l !== (prop?.label ?? "") ? { label: l } : {}),
      ...(target === "relation" ? relationFlags : {}),
    };
  });
  const body = $derived(target === "status" ? statusBody : target === "kind" ? kindBody : propBody);
  const changed = $derived(Object.keys(body).length > 0);
  const canSave = $derived(!busy && (adding ? name.trim() !== "" : changed) && !noParent);

  async function send(): Promise<void> {
    const q = { store };
    const n = editing as string;
    // The bodies are built by the target, so the casts only pick the one the target made.
    if (target === "status") {
      if (adding) await c.api.addStatus(statusBody as never, q);
      else await c.api.changeStatus(n, statusBody, q);
    } else if (target === "kind") {
      if (adding) await c.api.addKind(kindBody as never, q);
      else await c.api.changeKind(n, kindBody, q);
    } else if (adding) await c.api.addProp(propBody as never, q);
    else await c.api.changeProp(n, propBody, q);
  }

  async function save(): Promise<void> {
    if (!canSave) return;
    busy = true;
    err = null;
    try {
      const n = adding ? name.trim() : (editing as string);
      await send();
      await c.schema.reload(store);
      toast({ message: `${noun}「${n}」を${adding ? "追加" : "保存"}しました` });
      ondone();
      onclose();
    } catch (e) {
      err = e;
    } finally {
      busy = false;
    }
    if (err !== null) {
      await tick();
      form?.querySelector<HTMLElement>("[data-autofocus]")?.focus();
    }
  }

  function close(): void {
    if (!busy) onclose();
  }

  function onkeydown(e: KeyboardEvent): void {
    const t = e.target;
    if (
      e.key === "Enter" &&
      !e.isComposing &&
      e.keyCode !== 229 &&
      t instanceof HTMLInputElement &&
      t.type === "text"
    ) {
      e.preventDefault();
      void save();
    }
  }

  const failure = $derived(err === null ? null : failureOf(err));
  // The ID a refusal line starts with, and the rest of the line.
  const split = (line: string): [string, string] => {
    const i = line.indexOf(" ");
    return i < 0 ? [line, ""] : [line.slice(0, i), line.slice(i)];
  };

  const title = $derived(adding ? `${noun}を追加` : `${noun}「${editing}」を編集`);
  const categoryReason =
    "既定のステータスはカテゴリを変えられません。先に別のステータスを既定にしてください";
  const roleReason = "既定の種類は役割を変えられません。先に別の種類を既定にしてください";
</script>

<Dialog open {title} width={620} onclose={close}>
  <div class="form" role="presentation" bind:this={form} {onkeydown}>
    {#if adding}
      <div class="field">
        <label class="lbl" for="{uid}-name">名前</label>
        <input
          id="{uid}-name"
          type="text"
          autocomplete="off"
          data-autofocus
          disabled={busy}
          bind:value={name}
        >
      </div>
    {/if}
    <div class="field">
      <label class="lbl" for="{uid}-label">表示名</label>
      <input
        id="{uid}-label"
        type="text"
        autocomplete="off"
        data-autofocus={adding ? undefined : ""}
        disabled={busy}
        bind:value={label}
      >
    </div>

    {#if target === "status"}
      <div class="field">
        <label class="lbl" for="{uid}-cat">カテゴリ</label>
        <select
          id="{uid}-cat"
          disabled={busy || statusDefaultLocked}
          aria-describedby={statusDefaultLocked ? `${uid}-cat-why` : undefined}
          bind:value={category}
        >
          {#each CATEGORIES as k (k)}
            <option value={k}>{CATEGORY_LABEL[k]}</option>
          {/each}
        </select>
        {#if statusDefaultLocked}
          <p class="note" id="{uid}-cat-why">{categoryReason}</p>
        {/if}
      </div>
      <div class="field">
        <span class="lbl">色</span>
        <ColorPicker label="色" bind:value={color} original={status?.color ?? ""} />
      </div>
      {#if !adding}
        <div class="field">
          <label class="check">
            <input
              type="checkbox"
              disabled={busy || statusDefaultLocked}
              aria-describedby={statusDefaultLocked ? `${uid}-def-why` : undefined}
              bind:checked={isDefault}
            >
            既定
          </label>
          {#if statusDefaultLocked}
            <p class="note" id="{uid}-def-why">
              既定を外すには、別のステータスを既定にしてください
            </p>
          {/if}
        </div>
      {/if}
    {:else if target === "kind"}
      <div class="field">
        <label class="lbl" for="{uid}-role">役割</label>
        <select
          id="{uid}-role"
          disabled={busy || kindDefaultLocked}
          aria-describedby={kindDefaultLocked ? `${uid}-role-why` : undefined}
          bind:value={role}
        >
          <option value="container">container</option>
          <option value="work">work</option>
        </select>
        {#if kindDefaultLocked}
          <p class="note" id="{uid}-role-why">{roleReason}</p>
        {/if}
      </div>
      <fieldset class="field">
        <legend class="lbl">置ける親</legend>
        <label class="check">
          <input
            type="radio"
            name="{uid}-parents"
            disabled={busy}
            checked={anywhere}
            onchange={() => (anywhere = true)}
          >
          どこにでも
        </label>
        <label class="check">
          <input
            type="radio"
            name="{uid}-parents"
            disabled={busy}
            checked={!anywhere}
            onchange={() => (anywhere = false)}
          >
          指定する
        </label>
        {#if !anywhere}
          <div class="parents">
            {#each kindNames as p (p)}
              <label class="check">
                <input
                  type="checkbox"
                  disabled={busy}
                  checked={parents.includes(p)}
                  onchange={(e) => toggleParent(p, e.currentTarget.checked)}
                >
                {p === "root" ? "ルート" : p}
              </label>
            {/each}
          </div>
          {#if noParent}
            <p class="note">置ける親を 1 つ以上選んでください</p>
          {/if}
        {/if}
      </fieldset>
      <div class="field">
        <span class="lbl">アイコン</span>
        <IconPicker label="アイコン" bind:value={icon} original={kind?.icon ?? ""} {role} />
      </div>
      <div class="field">
        <label class="lbl" for="{uid}-tpl">既定のテンプレート</label>
        <select id="{uid}-tpl" disabled={busy} bind:value={template}>
          <option value="">なし</option>
          {#each templateNames as t (t)}
            <option value={t}>{t}</option>
          {/each}
        </select>
        {#if templatesFailure}
          <p class="note">テンプレートの一覧を読み込めません · {templatesFailure}</p>
        {/if}
      </div>
      {#if !adding && role === "work"}
        <div class="field">
          <label class="check">
            <input
              type="checkbox"
              disabled={busy || kindDefaultLocked}
              aria-describedby={kindDefaultLocked ? `${uid}-kdef-why` : undefined}
              bind:checked={isDefault}
            >
            既定
          </label>
          {#if kindDefaultLocked}
            <p class="note" id="{uid}-kdef-why">既定を外すには、別の種類を既定にしてください</p>
          {/if}
        </div>
      {/if}
    {:else}
      {#if adding && target === "prop"}
        <div class="field">
          <label class="lbl" for="{uid}-type">型</label>
          <select id="{uid}-type" disabled={busy} bind:value={propType}>
            {#each PROP_TYPES as t (t)}
              <option value={t}>{t}</option>
            {/each}
          </select>
        </div>
      {/if}
      {#if adding && target === "prop" && hasValues}
        <div class="field">
          <label class="lbl" for="{uid}-values">値（1 行に 1 つ）</label>
          <textarea id="{uid}-values" rows="4" disabled={busy} bind:value={valuesText}></textarea>
        </div>
      {/if}
      {#if target === "relation"}
        <div class="field">
          <label class="check">
            <input type="checkbox" disabled={busy} bind:checked={multi}>
            複数
          </label>
          <label class="check">
            <input type="checkbox" disabled={busy} bind:checked={symmetric}>
            双方向
          </label>
        </div>
        <div class="field">
          <label class="lbl" for="{uid}-inv">逆方向の表示名</label>
          <input
            id="{uid}-inv"
            type="text"
            autocomplete="off"
            disabled={busy || symmetric}
            bind:value={inverse}
          >
          {#if symmetric}
            <p class="note">双方向の関連には逆方向の表示名を付けられません</p>
          {/if}
        </div>
      {/if}
    {/if}

    {#if failure}
      {#if failure.violations}
        <div class="violations" role="alert">
          <p>この変更では、次のタスクがスキーマに合わなくなります。</p>
          <ul>
            {#each failure.violations as line (line)}
              {@const [id, rest] = split(line)}
              {@const ulid = ulidOf(id)}
              <li>
                {#if ulid}
                  {@const href = router.href({ name: "task", ulid })}
                  <a {href} onclick={(e) => onLinkClick(e, href)}>{id}</a>{rest}
                {:else}
                  {line}
                {/if}
              </li>
            {/each}
          </ul>
        </div>
      {:else}
        <FormError message={failure.text} />
      {/if}
    {/if}
  </div>

  {#snippet footer()}
    <Button disabled={busy} onclick={close}>キャンセル</Button>
    <Button variant="primary" disabled={!canSave} onclick={() => void save()}>
      {adding ? "追加" : "保存"}
    </Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  .lbl {
    padding: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .check {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
    font-size: var(--fs-13);
  }
  .parents {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6) var(--space-12);
    padding-left: var(--space-20, 20px);
  }
  input[type="text"],
  select,
  textarea {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-12);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-14);
  }
  textarea {
    padding: var(--space-8) var(--space-12);
    font-family: var(--font-mono);
    resize: vertical;
  }
  input[type="text"]:focus-visible,
  select:focus-visible,
  textarea:focus-visible {
    outline: 2px solid var(--accent);
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.6;
  }
  .violations {
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
  .violations p {
    margin: 0;
  }
  .violations ul {
    margin: var(--space-6) 0 0;
    padding-left: var(--space-20, 20px);
  }
  .violations a {
    color: inherit;
    font-family: var(--font-mono);
  }
</style>
