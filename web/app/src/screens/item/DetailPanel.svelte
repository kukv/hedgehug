<script lang="ts">
  import { readOnlyBand, readOnlyNote } from "@hedgehug/core";
  import { BlockedBadge, IconButton, Input, KindIcon, ReadOnlyBand, SidePanel } from "@hedgehug/ui";
  import { descriptionSaves, detail } from "../../wire";
  import { copyItemId } from "../../wireActions";
  import LinkSection from "../github/LinkSection.svelte";
  import CommentsSection from "../notes/CommentsSection.svelte";
  import DescriptionSection from "../notes/DescriptionSection.svelte";
  import HistorySection from "../notes/HistorySection.svelte";
  import DetailChildren from "./DetailChildren.svelte";
  import DetailHeader from "./DetailHeader.svelte";
  import DetailProperties from "./DetailProperties.svelte";
  import RelationsSection from "./RelationsSection.svelte";
  import WorklogSection from "./WorklogSection.svelte";

  // アイテムの詳細パネル(02「詳細パネル」のうち A2 の分。A2 の計画の決定 18)。
  const view = detail.view;
  const full = detail.full;

  // タイトルの入力欄。開いたアイテムが変わったら、そのタイトルに戻す。同じアイテムの読み直し(画面への
  // 反映)では、入力欄にフォーカスがあるあいだは書き換えない(06「編集中の入力欄の内容は書き換えない」)。
  // editing は $state にしない(フォーカスが外れたときに $effect が走り直し、保存の前に古い値へ戻すため)。
  let title = $state("");
  // base は、フォーカスしたときの値。打ち換えずに離れたら保存せず、反映で受け取った最新の値に合わせる。
  let editing = false;
  let base = "";
  let shownId: string | undefined;
  $effect(() => {
    const id = $view?.item.id;
    const next = $view?.item.title ?? "";
    if (id !== shownId || !editing) title = next;
    shownId = id;
  });

  async function save() {
    // 保存できなかった(空・断り)ときは、元のタイトルに戻す(02「タイトル」)。
    if (!(await detail.rename(title))) title = $view?.item.title ?? "";
  }

  function onkeydown(event: KeyboardEvent) {
    // Enter でもフォーカスを外して保存する。
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }
</script>

<SidePanel
  open={$view !== null}
  full={$full}
  label="詳細パネル"
  onclose={() => void detail.requestClose()}
>
  {#snippet head()}
    {#if $view}
      <DetailHeader v={$view} full={$full} />
    {/if}
  {/snippet}
  {#if $view}
    {@const v = $view}
    {#if v.readOnly}
      <ReadOnlyBand text={readOnlyBand(v.store.name)} inset />
    {/if}
    <div class="kind-row">
      <KindIcon kind={v.item.kind} />
      <span>{v.item.kind}</span>
      <code class="id">{v.displayId}</code>
      <IconButton label="ID をコピー" size="xs" onclick={() => void copyItemId(v.displayId)}
        >⧉</IconButton
      >
      {#if v.item.blocked}
        <BlockedBadge />
      {/if}
    </div>
    <Input
      label="タイトル"
      heading
      disabled={v.readOnly}
      title={v.readOnly ? readOnlyNote : undefined}
      onfocus={() => {
        editing = true;
        base = title;
      }}
      onblur={() => {
        editing = false;
        if (title === base) title = $view?.item.title ?? "";
        else void save();
      }}
      {onkeydown}
      bind:value={title}
    />
    <DetailProperties {v} />
    <!-- 開いたアイテムが変わったら、入力欄の状態ごと作り直す。反映(v の読み直し)では作り直さず、target だけ新しくなる。 -->
    {#key v.item.id}
      <DescriptionSection
        target={{ kind: "item", storeId: v.store.id, id: v.item.id }}
        readonly={v.readOnly}
      />
    {/key}
    {#if v.item.kind !== "Step"}
      <DetailChildren {v} />
    {/if}
    <RelationsSection {v} />
    <LinkSection {v} />
    <WorklogSection {v} />
    <CommentsSection
      target={{ kind: "item", storeId: v.store.id, id: v.item.id }}
      readonly={v.readOnly}
    />
    <HistorySection
      target={{ kind: "item", storeId: v.store.id, id: v.item.id }}
      refresh={[v.item, $descriptionSaves]}
    />
  {/if}
</SidePanel>

<style>
  .kind-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
  }
  .id {
    font-family: var(--font-mono);
    color: var(--muted);
  }
</style>
