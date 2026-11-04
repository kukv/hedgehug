// Screens this spec relies on (see screens.ts for the shared list):
//   - Onboarding, step 2 ("使う Store を選んでください"): the textbox "フォルダ" and the button
//     "探す" list the bare repositories under a folder that hold a Store; each row has a checkbox
//     "<path> を選ぶ", a textbox "<path> の呼び名" (the name the server proposes, the repo name),
//     and "マウントして次へ" mounts the chosen ones; step 3 lists them and "始める" closes Onboarding
//   - Settings › マウント中の Store has "Store を探す", a dialog with the same search; a Store that
//     is mounted already is not listed again ("見つかりませんでした")
import { join } from "node:path";
import { expect, Peer, test } from "./harness";

test.use({ store: null });

test("two Stores in a folder are found, one is renamed, and both are mounted", async ({
  page,
  hh,
}) => {
  const dir = join(hh.home, "remotes");
  const peers = [
    Peer.publish("alpha", "AL", join(dir, "alpha.git")),
    Peer.publish("beta", "BE", join(dir, "beta.git")),
  ];
  try {
    await hh.open(page);
    await page.getByRole("button", { name: "次へ: Store を探す" }).click();
    await page.getByLabel("フォルダ", { exact: true }).fill(dir);
    await page.getByRole("button", { name: "探す", exact: true }).click();

    const alpha = page.getByLabel(/alpha\.git を選ぶ/);
    const beta = page.getByLabel(/beta\.git を選ぶ/);
    await expect(alpha).toBeVisible();
    await expect(beta).toBeVisible();
    await expect(page.getByLabel(/alpha\.git の呼び名/)).toHaveValue("alpha");
    await expect(page.getByLabel(/beta\.git の呼び名/)).toHaveValue("beta");

    await page.getByLabel(/beta\.git の呼び名/).fill("beta-renamed");
    await alpha.check();
    await beta.check();
    await page.getByRole("button", { name: "マウントして次へ" }).click();

    await expect(page.getByRole("heading", { name: "Store を使い始めます" })).toBeVisible();
    const mounted = page.getByRole("list", { name: "マウントした Store" });
    await expect(mounted.getByText("alpha", { exact: true })).toBeVisible();
    await expect(mounted.getByText("beta-renamed", { exact: true })).toBeVisible();
    const names = hh.json<{ name: string }[]>("store", "list").map((s) => s.name);
    expect(names.sort()).toEqual(["alpha", "beta-renamed"]);

    await page.getByRole("button", { name: "始める" }).click();
    await expect(page.getByText("Hedgehug へようこそ")).toBeHidden();
    await expect(page.getByRole("link", { name: /alpha/ }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /beta/ }).first()).toBeVisible();

    // The Settings look in the same folder: what is mounted is not offered again.
    await hh.open(page, "/settings/stores");
    await page.getByRole("button", { name: "Store を探す" }).click();
    const dialog = page.getByRole("dialog", { name: "Store を探す" });
    await dialog.getByLabel("フォルダ", { exact: true }).fill(dir);
    await dialog.getByRole("button", { name: "探す", exact: true }).click();
    await expect(dialog.getByText("見つかりませんでした")).toBeVisible();
  } finally {
    for (const p of peers) p.stop();
  }
});
