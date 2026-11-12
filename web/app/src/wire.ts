import { createSystemPort } from "@hedgehug/api";
import { createShell, createSystemInfo } from "@hedgehug/core";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。
export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
