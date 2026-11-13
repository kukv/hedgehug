import type { Member } from "../../model/member";

// メンション(02「メンション」)。ログイン名は、大文字と小文字を区別せずに比べる(01)。
// `@ログイン名` をメンションの見た目にする判定 isMemberOf は、Markdown の字句分け(Task 5)にある。

// mentionTargets は、コメントの入力欄の下に並べるメンションのボタンのログイン名(あなた以外のメンバー。A4a-23)。
export function mentionTargets(members: Member[], me: string): string[] {
  return members.filter((m) => m.login.toLowerCase() !== me.toLowerCase()).map((m) => m.login);
}

// appendMention は、入力の末尾に `@ログイン名 ` を足す。
export function appendMention(text: string, login: string): string {
  return `${text}@${login} `;
}
