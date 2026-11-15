<script lang="ts">
  import { clockWarning } from "@hedgehug/core";
  import { sync } from "../../wire";

  // 設定の「一般」(08)。C5d では時計のずれの警告だけを置く。ずれがなければ中は空(C5d-1)。
  // G がログイン名をこの警告の上に、同期のタイミングを下に足す。
  const status = sync.status;
  const warning = $derived(clockWarning($status));
</script>

{#if warning}
  <p class="warn">{warning}</p>
{/if}

<style>
  .warn {
    margin: 0;
    color: var(--c-amber-fg);
  }
</style>
