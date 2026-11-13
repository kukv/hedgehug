<script lang="ts">
  import {
    closeBody,
    closeDefault,
    closeDestinations,
    keptDestination,
    type PlanningOverview,
    type SprintCard,
    sprintClosed,
  } from "@hedgehug/core";
  import { Button, Dialog, Select } from "@hedgehug/ui";
  import { sprintAction } from "../../planning";
  import { planningPort } from "../../wire";

  // スプリントの完了のダイアログ(05「完了」)。未完了が 1 件以上のときだけ、移動先を選ばせる。
  interface Props {
    storeId: string;
    workspaceId: string;
    card: SprintCard;
    overview: PlanningOverview;
    onclose: () => void;
  }

  let { storeId, workspaceId, card, overview, onclose }: Props = $props();

  const s = $derived(card.sprint);
  const label = $derived(`未完了の ${card.undone} 件の移動先`);
  // セレクトの値は文字列なので、バックログ(null)は "" にする。スプリントの ID は空にならない。
  const options = $derived(
    closeDestinations(overview, s.id).map((d) => ({ value: d.id ?? "", label: d.label })),
  );
  const initial = () => closeDefault(overview, card.sprint.id) ?? "";
  let destination = $state(initial());
  // 概要の読み直しで、選んでいたスプリントが選択肢から消えたら、既定に戻す。
  $effect(() => {
    const kept = keptDestination(overview, s.id, destination === "" ? null : destination) ?? "";
    if (kept !== destination) destination = kept;
  });
  let error = $state("");
  let busy = $state(false);

  async function save() {
    if (busy) return;
    busy = true;
    const { id, name } = s;
    const to = card.undone > 0 && destination !== "" ? destination : null;
    const ok = await sprintAction(
      async () => {
        await planningPort.closeSprint(storeId, workspaceId, id, to);
        return sprintClosed(name);
      },
      (message) => (error = message),
    );
    busy = false;
    if (ok) onclose();
  }
</script>

<Dialog open title={s.name} {onclose}>
  <div class="form">
    <p class="text">{closeBody(card)}</p>
    {#if card.undone > 0}
      <div class="field">
        <span>{label}</span>
        <Select {label} {options} bind:value={destination} />
      </div>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>キャンセル</Button>
    <Button variant="primary" disabled={busy} onclick={save}>完了</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .text {
    margin: 0;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
  .grow {
    flex: 1;
  }
</style>
