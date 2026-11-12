<script lang="ts">
  import { type SettingsState, valueListLabel } from "@hedgehug/core";
  import { Button, Dialog } from "@hedgehug/ui";
  import ValuesPanel from "./ValuesPanel.svelte";

  // 単一・複数選択の選択肢の編集(03「プロパティ」)。行と追加の欄は値の編集と同じで、list にプロパティの ID を渡す。
  interface Props {
    state: SettingsState;
    propertyId: string;
    onclose: () => void;
  }

  let { state: s, propertyId, onclose }: Props = $props();
</script>

<Dialog open wide title={valueListLabel(s.detail.settings, propertyId)} {onclose}>
  <ValuesPanel state={s} list={propertyId} />
  {#snippet footer()}
    <span class="grow"></span>
    <Button onclick={onclose}>閉じる</Button>
  {/snippet}
</Dialog>

<style>
  .grow {
    flex: 1;
  }
</style>
