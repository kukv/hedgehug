import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createProjectTransfer } from "./projectDialog";
import { fakeTransfers } from "./transfers.test-util";

// projectDialog.test.ts と projectDialogSubmit.test.ts の共通の準備。
// カタログは Store work(S1、書ける)と ro(S2、閲覧のみ)。候補は S1 の「Mobile」と S3(team)の「Team」。

export async function setupProject() {
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
  fake.projectTargetList = [
    { storeId: "S1", storeName: "work", workspaceId: "S1-W2", workspaceName: "Mobile" },
    { storeId: "S3", storeName: "team", workspaceId: "S3-W1", workspaceName: "Team" },
  ];
  const toastList = createToasts({ set: () => 0, clear: () => {} });
  const moved: string[] = [];
  const transfer = createProjectTransfer({
    transfers: fake,
    catalog: reloading,
    toasts: toastList,
    online: () => env.online,
    afterMoved: async (result, project) => {
      moved.push(`${project.id} → ${result.toStore}/${result.display}`);
    },
  });
  const project = { id: "S1-P1", storeId: "S1", title: "決済基盤" };
  const roProject = { id: "S2-P1", storeId: "S2", title: "決済基盤" };
  const toasts = () => get(toastList.list).map((t) => ({ text: t.text, error: t.error }));
  return { transfer, fake, env, moved, project, roProject, toasts };
}
