<script lang="ts">
  import {
    ownerName,
    type ProjectStatus,
    type ProjectView,
    projectStatuses,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Input, Select } from "@hedgehug/ui";
  import { projectEditor } from "../../projectEditor";

  // 概要の項目の編集(02「概要の編集」)。1 項目を変えるごとに 1 回保存する。検査と断りは core の projectEditor が持つ。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const disabled = $derived(view.store.readOnly);
  const hint = $derived(disabled ? readOnlyNote : undefined);
  const statuses = projectStatuses.map((s) => ({ value: s, label: s }));
  // オーナーは「なし」とメンバー。メンバーにいない人がオーナーのままのときは、その人も選択肢に出す。
  const owners = $derived([
    { value: "", label: "なし" },
    ...view.members.map((m) => ({ value: m.login, label: m.name })),
    ...(p.owner !== "" && !view.members.some((m) => m.login.toLowerCase() === p.owner.toLowerCase())
      ? [{ value: p.owner, label: ownerName(p.owner, view.members) }]
      : []),
  ]);

  // 入力欄の値。フォーカスしている欄は、画面への反映(保存して読み直したもの)で書き換えない(06)。
  // editing は $state にしない(フォーカスが外れたときに $effect が走り直し、保存の前に古い値へ戻すため)。
  const fields = $state({ title: "", start: "", end: "" });
  let editing = "";
  $effect(() => {
    const saved = { title: p.title, start: p.start, end: p.end };
    for (const key of ["title", "start", "end"] as const) {
      if (editing !== key) fields[key] = saved[key];
    }
  });

  // 保存できなかった(空・断り)ときは、保存済みの値に戻す。
  async function saveTitle() {
    editing = "";
    if (!(await projectEditor.rename(p, fields.title))) fields.title = p.title;
  }

  async function saveDate(key: "start" | "end") {
    editing = "";
    if (!(await projectEditor.setDate(p, key, fields[key]))) fields[key] = p[key];
  }

  // Enter でもフォーカスを外して保存する。日本語の変換を確定する Enter は除く。
  function onTitleKey(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }
</script>

<section class="props" aria-label="プロパティ">
  <div class="prop">
    <span class="prop-k">タイトル</span>
    <Input
      label="タイトル"
      size="sm"
      {disabled}
      title={hint}
      bind:value={fields.title}
      onfocus={() => (editing = "title")}
      onblur={() => void saveTitle()}
      onkeydown={onTitleKey}
    />
  </div>
  <div class="prop">
    <span class="prop-k">ステータス</span>
    <Select
      label="ステータス"
      size="sm"
      options={statuses}
      value={p.status}
      {disabled}
      title={hint}
      onchange={(v) => void projectEditor.setStatus(p, v as ProjectStatus)}
    />
  </div>
  <div class="prop">
    <span class="prop-k">オーナー</span>
    <Select
      label="オーナー"
      size="sm"
      options={owners}
      value={p.owner}
      {disabled}
      title={hint}
      onchange={(v) => void projectEditor.setOwner(p, v)}
    />
  </div>
  <div class="prop">
    <span class="prop-k">開始日</span>
    <Input
      label="開始日"
      type="date"
      size="sm"
      {disabled}
      title={hint}
      bind:value={fields.start}
      onfocus={() => (editing = "start")}
      onblur={() => void saveDate("start")}
    />
  </div>
  <div class="prop">
    <span class="prop-k">終了日</span>
    <Input
      label="終了日"
      type="date"
      size="sm"
      {disabled}
      title={hint}
      bind:value={fields.end}
      onfocus={() => (editing = "end")}
      onblur={() => void saveDate("end")}
    />
  </div>
  <p class="note">プロジェクトのプロパティは固定です。ステータスの値も変更できません。</p>
</section>

<style>
  .props {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .prop {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr);
    align-items: center;
  }
  .prop-k {
    color: var(--muted);
    font-size: 12.5px;
  }
  .note {
    margin: 4px 0 0;
    color: var(--muted);
    font-size: 12px;
  }
</style>
