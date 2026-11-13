<script lang="ts">
  import { type DetailView, watchView } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { watch } from "../../wire";

  // ヘッダーのウォッチのボタン(02「ウォッチ」)。いまウォッチ中かどうかが、文字と aria-pressed で分かる。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const view = $derived(watchView(v.item, v.people.me, v.readOnly));
  const busy = watch.busy;
</script>

<Button
  size="sm"
  pressed={view.watching}
  disabled={view.disabled || $busy}
  title={view.note}
  onclick={() => void watch.toggle(v.item, v.people.me)}
  >{view.label}</Button
>
