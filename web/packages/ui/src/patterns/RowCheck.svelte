<script lang="ts">
  // 一覧の行の先頭に置く、選択のチェック(F2b1-4)。見える文字はなく、label は読み上げの名前(「{ID} を選択」)。
  // 閲覧のみの Store でも選べるので、無効にしない。
  interface Props {
    label: string;
    checked: boolean;
    onchange: (checked: boolean) => void;
  }

  let { label, checked, onchange }: Props = $props();
</script>

<input
  type="checkbox"
  aria-label={label}
  {checked}
  onchange={(e) => onchange(e.currentTarget.checked)}
>
