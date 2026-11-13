import { type ChildProcess, spawn } from "node:child_process";
import { resolve } from "node:path";
import { test as base, expect } from "@playwright/test";

// worker ごとに、e2e 用にビルドしたサーバー(bin/hedgehug-e2e)を空いた番号で起動する。
// worker どうしでデータが混ざらず、ほかの worktree のセッションのサーバーとも番号が重ならない。

const bin = resolve(import.meta.dirname, "../../bin/hedgehug-e2e");
const startTimeout = 15_000;
const stopTimeout = 5_000;

// start は、サーバーを起動し、標準出力の 1 行 `http://127.0.0.1:<番号>/#token=...`(cmd/hedgehug/main.go)から
// 番号までの URL を取り出して返す。時間内に出ないか先に終わったら、標準エラーを付けて落とし、SIGKILL する。
// 起動できたあとの標準エラーは、サーバー側の panic などを CI のログに残すため、接頭辞を付けて流す。
function start(): Promise<{ proc: ChildProcess; url: string }> {
  const proc = spawn(bin, [], {
    env: { ...process.env, HEDGEHUG_ADDR: "127.0.0.1:0", HEDGEHUG_TOKEN: "e2e-token" },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let stdout = "";
  let stderr = "";
  let started = false;
  proc.stderr?.on("data", (chunk) => {
    if (!started) {
      stderr += chunk;
      return;
    }
    process.stderr.write(String(chunk).replace(/^(?=.)/gm, "[hedgehug-e2e] "));
  });
  return new Promise((done, fail) => {
    const failWith = (reason: string) => {
      clearTimeout(timer);
      proc.kill("SIGKILL");
      fail(new Error(`e2e のサーバーが起動しない(${reason})。標準エラー:\n${stderr}`));
    };
    const timer = setTimeout(
      () => failWith(`${startTimeout / 1000} 秒以内に URL が出ない`),
      startTimeout,
    );
    // exit では標準エラーを読み切れていないことがあるので、close(入出力が閉じたあと)で判定する。
    const onClose = (code: number | null, signal: NodeJS.Signals | null) =>
      failWith(`先に終了した。code=${code} signal=${signal}`);
    const onError = (err: Error) => failWith(err.message);
    proc.on("error", onError);
    proc.on("close", onClose);
    proc.stdout?.on("data", (chunk) => {
      stdout += chunk;
      const m = stdout.match(/^(http:\/\/127\.0\.0\.1:\d+)\/#token=/m);
      if (m?.[1]) {
        clearTimeout(timer);
        started = true;
        stderr = "";
        // 起動後の error(終了時の kill の失敗など)で、動いているサーバーを落とさない。
        proc.off("error", onError);
        proc.off("close", onClose);
        done({ proc, url: m[1] });
      }
    });
  });
}

// stop は、SIGINT を送り、5 秒で終わらなければ SIGKILL する。
async function stop(proc: ChildProcess) {
  if (proc.exitCode !== null || proc.signalCode !== null) return;
  const exited = new Promise((done) => proc.once("exit", done));
  proc.kill("SIGINT");
  const timer = setTimeout(() => proc.kill("SIGKILL"), stopTimeout);
  await exited;
  clearTimeout(timer);
}

export const test = base.extend<object, { server: { url: string } }>({
  server: [
    // biome-ignore lint/correctness/noEmptyPattern: Playwright は fixture の第 1 引数に分割代入の形を求める。
    async ({}, use) => {
      const { proc, url } = await start();
      try {
        await use({ url });
      } finally {
        await stop(proc);
      }
    },
    { scope: "worker" },
  ],
  baseURL: async ({ server }, use) => use(server.url),
});

export { expect };
