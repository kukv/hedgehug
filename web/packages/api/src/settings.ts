import type { SettingsPort } from "@hedgehug/core";
import { call } from "./errors";
import { getLogin, getSyncTiming, previewLogin, setLogin, setSyncTiming } from "./generated";

// マシンの設定の口(08「一般」のログイン名と同期のタイミング)。
export function createSettingsPort(): SettingsPort {
  return {
    login: () => call(async () => (await getLogin()).login),
    previewLogin: (login) => call(async () => (await previewLogin({ login })).lines),
    setLogin: (login) => call(() => setLogin({ login })),
    timing: () =>
      call(async () => {
        const t = await getSyncTiming();
        return {
          auto: t.auto,
          intervalMinutes: t.intervalMinutes,
          afterWriteSeconds: t.afterWriteSeconds,
          githubMinutes: t.githubMinutes,
        };
      }),
    setTiming: (timing) => call(() => setSyncTiming(timing)),
  };
}
