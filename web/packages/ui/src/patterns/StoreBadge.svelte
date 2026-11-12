<script lang="ts">
  interface Props {
    name: string;
    // color は、トークン --t-<色>-bg・--t-<色>-fg の色の名前(teal・orange・purple・sky・pink・olive・brown)。
    color: string;
  }

  let { name, color }: Props = $props();
</script>

<span class="store-badge" style="--tbg: var(--t-{color}-bg); --tfg: var(--t-{color}-fg)">
  {name}
</span>

<style>
  .store-badge {
    display: inline-flex;
    align-items: center;
    height: 20px;
    padding: 0 7px;
    border-radius: 5px;
    font-size: 11.5px;
    font-weight: 500;
    font-family: var(--font-mono);
    background: var(--tbg);
    color: var(--tfg);
    white-space: nowrap;
  }
</style>
