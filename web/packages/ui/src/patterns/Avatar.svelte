<script lang="ts">
  // アバター(決定 24)。name の頭の 1 文字を丸に出す。name がなければ中身のない丸(01「不明なユーザー」)。
  interface Props {
    name?: string;
    title: string;
  }

  let { name = "", title }: Props = $props();
</script>

<span class="avatar" class:empty={name === ""} role="img" {title} aria-label={title}
  >{name.slice(0, 1)}</span
>

<style>
  .avatar {
    display: inline-grid;
    place-items: center;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--c-slate-bg);
    font-size: 10.5px;
  }
  .empty {
    background: none;
    border: 1px dashed var(--muted);
  }
</style>
