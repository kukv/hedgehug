<script lang="ts">
  // 進捗の棒と「N%」(04「集計の定義」)。棒の上にポインターを置くと detail(「{完了} / {対象}」)が出る。
  interface Props {
    percent: number;
    detail: string;
  }

  let { percent, detail }: Props = $props();
</script>

<span class="progress">
  <span
    class="bar"
    role="progressbar"
    aria-label="進捗"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={percent}
    aria-valuetext={detail}
    title={detail}
  >
    <span class="fill" style:width="{percent}%"></span>
  </span>
  <span class="pct">{percent}%</span>
</span>

<style>
  .progress {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex: none;
  }
  .bar {
    width: 56px;
    height: 6px;
    border-radius: 3px;
    background: var(--line);
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .pct {
    min-width: 3ch;
    text-align: right;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
</style>
