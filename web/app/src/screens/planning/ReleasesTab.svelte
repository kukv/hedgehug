<script lang="ts">
  import {
    type ReleaseCard as Card,
    dayOf,
    newReleaseForm,
    type ReleaseState,
    readOnlyNote,
    releaseConfirm,
    releaseForm,
    releaseStateChanged,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog } from "@hedgehug/ui";
  import { releaseAction, releases } from "../../planning";
  import { planning, planningPort, syncNow, toasts } from "../../wire";
  import DeleteReleaseDialog from "./DeleteReleaseDialog.svelte";
  import ReleaseCard from "./ReleaseCard.svelte";
  import ReleaseDialog from "./ReleaseDialog.svelte";

  // ワークスペースの「リリース」のタブ(05「リリース」)。作成・編集・削除・状態の変更のダイアログもここで出す。
  interface Props {
    storeId: string;
    storeName: string;
    workspaceId: string;
    readOnly: boolean;
  }

  let { storeId, storeName, workspaceId, readOnly }: Props = $props();

  // タブを開いているあいだだけ読む。Store の出来事で読み直す(planning.ts)。
  $effect(() => {
    void releases.open(storeId, workspaceId);
    return () => releases.close();
  });
  const loaded = releases.state;
  const overview = planning.state;
  const view = $derived(
    $loaded && $loaded.storeId === storeId && $loaded.workspaceId === workspaceId ? $loaded : null,
  );
  // 削除の確認の保持日数は、概要から読む(WorkspaceScreen が開いている)。
  const ov = $derived(
    $overview && $overview.storeId === storeId && $overview.workspaceId === workspaceId
      ? $overview.overview
      : null,
  );
  // 日付は、時刻の更新(30 秒ごと)で読み直す。日付が変わると「予定日を過ぎています」が変わる(01「日付と時刻」)。
  const today = $derived(dayOf($syncNow));
  const thisYear = $derived($syncNow.getFullYear());
  const note = $derived(readOnly ? readOnlyNote : undefined);

  // 開いたワークスペースのダイアログだけ出す(別のワークスペースへ移ったら閉じる)。削除は概要の保持日数を
  // 使うので、まだ読めていなければ読み直してから出す。読み直しても読めなければ出さない(A2 の決定 29)。
  type Kind = "create" | "edit" | "delete" | "confirm";
  let dialog = $state<{ kind: Kind; card: Card | null; wid: string } | null>(null);
  async function openDialog(kind: Kind, card: Card | null = null) {
    const opening = workspaceId;
    if (kind === "delete" && !ov) await planning.reload();
    if (opening === workspaceId && (kind !== "delete" || ov)) dialog = { kind, card, wid: opening };
  }
  const closeDialog = () => (dialog = null);
  const shown = $derived(dialog && dialog.wid === workspaceId ? dialog : null);
  $effect(() => {
    if (dialog && !shown) dialog = null;
  });
  const card = $derived(shown?.card ?? null);
  // 確認はいまのカードで出す。リリースか未完了がなくなったら、保存せずに閉じる(押し直せば確認なしで変わる)。
  const confirming = $derived(
    shown?.kind === "confirm"
      ? (view?.cards.find((c) => c.release.id === shown.card?.release.id) ?? null)
      : null,
  );
  const confirmText = $derived(confirming ? releaseConfirm(confirming) : null);
  $effect(() => {
    if (shown?.kind === "confirm" && !confirmText) dialog = null;
  });

  // 状態の変更。リリース済みにするときだけ、未完了が残れば確認する(05「状態」)。断りはエラーのトースト。
  // 保存中のリリースの ID。同じリリースは重ねて変えない(ほかのカードは変えられる)。
  let changing = $state<Record<string, boolean>>({});
  function onstate(c: Card, to: ReleaseState) {
    if (to === "released" && releaseConfirm(c)) void openDialog("confirm", c);
    else void change(c, to);
  }
  async function change(c: Card, to: ReleaseState) {
    const { id, name } = c.release;
    if (changing[id]) return;
    changing[id] = true;
    await releaseAction(
      async () => {
        await planningPort.setReleaseState(storeId, workspaceId, id, to);
        return releaseStateChanged(name, to);
      },
      (message) => toasts.show(message, { error: true }),
    );
    delete changing[id];
  }
  function confirmed() {
    const c = confirming;
    dialog = null;
    if (c) void change(c, "released");
  }
</script>

<div class="top">
  <Button
    variant="primary"
    disabled={readOnly}
    title={note}
    onclick={() => void openDialog("create")}
    >リリースを作成</Button
  >
</div>
{#if view}
  {#if view.cards.length === 0}
    <div class="empty">
      <p class="title">リリースはまだありません</p>
      <p>バージョンを作って Epic・Story・Task・Bug をまとめると、何が残っているかが見えます。</p>
    </div>
  {:else}
    <div class="cards">
      {#each view.cards as c (c.release.id)}
        <ReleaseCard
          card={c}
          {storeId}
          {storeName}
          settings={view.settings}
          members={view.members}
          {readOnly}
          saving={changing[c.release.id] ?? false}
          {today}
          {thisYear}
          {onstate}
          onedit={(x) => void openDialog("edit", x)}
          ondelete={(x) => void openDialog("delete", x)}
        />
      {/each}
    </div>
  {/if}
{/if}
{#if shown?.kind === "create"}
  <ReleaseDialog
    {storeId}
    {workspaceId}
    release={null}
    initial={newReleaseForm()}
    onclose={closeDialog}
  />
{:else if shown?.kind === "edit" && card}
  <ReleaseDialog
    {storeId}
    {workspaceId}
    release={card.release}
    initial={releaseForm(card.release)}
    onclose={closeDialog}
  />
{:else if shown?.kind === "delete" && card && ov}
  <DeleteReleaseDialog
    {storeId}
    {workspaceId}
    {card}
    retentionDays={ov.retentionDays}
    onclose={closeDialog}
  />
{:else if confirmText}
  <ConfirmDialog
    title="リリース済みにする"
    lines={[confirmText]}
    ok="リリース済みにする"
    onok={confirmed}
    oncancel={closeDialog}
  />
{/if}

<style>
  .top {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 8px;
  }
  .cards {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 48px 16px;
    text-align: center;
    color: var(--muted);
  }
  .empty p {
    margin: 0;
  }
  .empty .title {
    font-size: 15px;
    color: var(--ink);
  }
</style>
