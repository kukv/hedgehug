<script lang="ts">
  import { propertyKinds } from "@hedgehug/core";
  import { Button, Checkbox, KindIcon } from "@hedgehug/ui";

  // 使う種類のチェックメニュー(03「使う種類」)。チェックのたびに保存し、開いたままにする。
  interface Props {
    x: number;
    y: number;
    kinds: string[] | "all";
    readOnly: boolean;
    ontoggle: (kind: string) => Promise<unknown>;
    onclose: () => void;
  }

  let { x, y, kinds, readOnly, ontoggle, onclose }: Props = $props();

  let element: HTMLFieldSetElement | undefined = $state();
  // 断りで保存されなかったとき、チェックの見た目を保存済みの状態に戻すために作り直す。
  let round = $state(0);

  const checked = (kind: string) => kinds === "all" || kinds.includes(kind);

  async function toggle(kind: string) {
    await ontoggle(kind);
    round += 1;
  }
  function onpointerdown(event: PointerEvent) {
    if (element && !element.contains(event.target as Node)) onclose();
  }
  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Escape") onclose();
  }
</script>

<svelte:window {onpointerdown} {onkeydown} />

<fieldset
  class="pop"
  aria-label="使う種類"
  style:left="{x}px"
  style:top="{y}px"
  bind:this={element}
>
  {#key round}
    {#each propertyKinds as kind (kind)}
      <div class="line">
        <Checkbox
          label={kind}
          disabled={readOnly}
          bind:checked={() => checked(kind), () => void toggle(kind)}
        />
        <KindIcon {kind} />
      </div>
    {/each}
  {/key}
  {#if readOnly}
    <Button size="sm" onclick={onclose}>閉じる</Button>
  {/if}
</fieldset>

<style>
  .pop {
    position: fixed;
    z-index: 60;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: 10px;
    box-shadow: var(--shadow-pop);
    margin: 0;
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .line {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
</style>
