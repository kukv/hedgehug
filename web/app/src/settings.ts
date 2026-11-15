import { createSettingsPort } from "@hedgehug/api";
import { createLoginField, createSyncTimingField } from "@hedgehug/core";
import { reloads, syncReloads } from "./reloads";
import { navigation } from "./wire";
import { toasts } from "./wireBase";

// 設定「一般」のログイン名と同期のタイミング(08)。wire.ts が 300 行なので、ここで作る。
const settingsPort = createSettingsPort();
export const loginField = createLoginField(settingsPort, toasts);
export const syncTiming = createSyncTimingField(settingsPort, toasts);
// ほかのタブで変えたら読み直す。ログイン名の変更は store の知らせ、同期のタイミングの変更は sync の知らせで届く(G1-17)。
reloads.add(() => loginField.load());
syncReloads.add(() => syncTiming.load());

// 画面を移ったら、「ログイン名を変更」の確認を閉じて欄を元の値に戻す(01「画面の移り方」。wire.ts のほかのダイアログと同じ)。
navigation.route.subscribe(() => loginField.cancel());
