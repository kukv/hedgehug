import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeTransfers } from "./transfers.test-util";
import { createWorkspaceTransfer } from "./workspaceDialog";

// workspaceDialog.test.ts と workspaceDialogSubmit.test.ts の共通の準備。
// カタログは Store work(S1、書ける)と ro(S2、閲覧のみ)。候補は S3(team)と S4(far)。

export async function setupWorkspace() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  // reloads は、カタログを読み直した回数(load の前の 1 回は数えない)。
  const env = { online: true, reloads: 0 };
  const reloading = {
    store: catalog.store,
    load: async () => {
      env.reloads += 1;
    },
  };
  const fake = fakeTransfers();
  fake.workspaceTargetList = {
    targets: [
      { storeId: "S3", storeName: "team" },
      { storeId: "S4", storeName: "far" },
    ],
    hiddenOnly: false,
  };
  const toastList = createToasts({ set: () => 0, clear: () => {} });
  const moved: string[] = [];
  const pressed: string[] = [];
  const transfer = createWorkspaceTransfer({
    transfers: fake,
    catalog: reloading,
    toasts: toastList,
    online: () => env.online,
    afterMoved: (ws) => {
      pressed.push(ws.id);
      return async (result) => {
        moved.push(`${ws.id} → ${result.toStoreName}/${result.prefix}`);
      };
    },
  });
  const workspace = { id: "S1-W1", storeId: "S1", name: "Web", prefix: "HH" };
  const roWorkspace = { id: "S2-W1", storeId: "S2", name: "Web", prefix: "HH" };
  const toasts = () => get(toastList.list).map((t) => ({ text: t.text, error: t.error }));
  return { transfer, fake, env, moved, pressed, workspace, roWorkspace, toasts };
}
