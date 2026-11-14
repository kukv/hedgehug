<script lang="ts">
  import { type BacklogRow, pointsToSend, pt, readOnlyNote } from "@hedgehug/core";
  import { backlog } from "../../planning";

  // バックログの行のポイントの入力(05「行」、F2a-8)。確定(Enter かフォーカスを外す)で保存する。空は値の削除。
  // 値の検査はサーバーが断り、そのときは保存済みの値に戻す。
  interface Props {
    row: BacklogRow;
    disabled: boolean;
  }

  let { row, disabled }: Props = $props();

  const saved = $derived(row.points === null ? "" : pt(row.points));
  // editing は、入力欄にフォーカスがあるあいだ。画面への反映では、編集中の入力欄を書き換えない(06)。
  // base は、フォーカスしたときの値。
  let editing = false;
  let base = "";
  let input: HTMLInputElement | undefined = $state();
  $effect(() => {
    const next = saved;
    if (!editing && input) input.value = next;
  });

  // 入力欄の値は、入力欄そのものから文字列で読む。数でない文字は、サーバーに断らせる(PropertyField と同じ)。
  async function commit() {
    if (!input) return;
    editing = false;
    const target = input;
    const text = pointsToSend(target.value, target.validity?.badInput ?? false, base);
    // 打ち換えずに離れたら保存せず、反映で受け取った最新の値に合わせる。
    if (text === null) {
      target.value = saved;
      return;
    }
    // 断られたら保存済みの値に戻す。保存できたときも、読み直した値の書き方(「1.50」は「1.5」)にそろえる。
    // 読み直しで値が変わらなければ、上の $effect は走り直さないので、ここで書く。
    await backlog.setPoints(row, text);
    if (!editing) target.value = saved;
  }
  function enter(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }
</script>

<input
  bind:this={input}
  class="points"
  type="number"
  step="any"
  min="0"
  aria-label="ポイント"
  {disabled}
  title={disabled ? readOnlyNote : undefined}
  onfocus={(e) => {
    editing = true;
    base = e.currentTarget.value;
  }}
  onblur={() => void commit()}
  onkeydown={enter}
>

<style>
  .points {
    width: 64px;
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 8px;
  }
  .points:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
