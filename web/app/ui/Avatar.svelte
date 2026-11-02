<script lang="ts">
  import { avatarTone, initials } from "../../core/format";

  type Props = { name: string; size?: 24 | 32 };
  let { name, size = 24 }: Props = $props();
  const tone = $derived(avatarTone(name));
</script>

<span
  class="avatar"
  class:lg={size === 32}
  data-tone={tone}
  role="img"
  aria-label={name}
  title={name}
>
  {initials(name)}
</span>

<style>
  .avatar {
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: 999px;
    font-size: 10px;
    font-weight: 600;
  }
  .lg {
    width: 32px;
    height: 32px;
    font-size: var(--fs-mono);
  }
  .avatar[data-tone="0"] {
    background: var(--avatar-0-bg);
    color: var(--avatar-0-fg);
  }
  .avatar[data-tone="1"] {
    background: var(--avatar-1-bg);
    color: var(--avatar-1-fg);
  }
  .avatar[data-tone="2"] {
    background: var(--avatar-2-bg);
    color: var(--avatar-2-fg);
  }
  .avatar[data-tone="3"] {
    background: var(--avatar-3-bg);
    color: var(--avatar-3-fg);
  }
</style>
