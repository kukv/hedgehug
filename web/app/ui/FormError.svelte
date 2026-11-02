<script lang="ts">
  // The red band of a dialog that shows the words of a refusal.
  let { message }: { message: string } = $props();
</script>

<p class="failure" role="alert">{message}</p>

<style>
  .failure {
    margin: 0;
    padding: var(--space-10) var(--space-12);
    border-radius: var(--radius-10);
    background: var(--danger-weak);
    color: var(--danger-ink);
    line-height: 1.6;
  }
</style>
