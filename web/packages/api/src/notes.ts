import type { Comment, NotesPort, Worklog } from "@hedgehug/core";
import { call } from "./errors";
import {
  type Comment as ApiComment,
  type Worklog as ApiWorklog,
  addWorklog,
  deleteItemComment,
  deleteProjectComment,
  deleteWorklog,
  describeItem,
  describeProject,
  editItemComment,
  editProjectComment,
  getItemDescription,
  getProjectDescription,
  listItemComments,
  listProjectComments,
  listWorklogs,
  postItemComment,
  postProjectComment,
} from "./generated";
import { relationCalls } from "./relationsPort";

function toComment(c: ApiComment): Comment {
  return { id: c.id, by: c.by, byName: c.byName, at: c.at, body: c.body, edited: c.edited };
}

function toWorklog(w: ApiWorklog): Worklog {
  return { id: w.id, by: w.by, byName: w.byName, date: w.date, hours: w.hours, memo: w.memo };
}

// 説明・コメント・作業時間・関連・履歴の口の実装。区画ごとのタスクが、自分のメソッドをここに足す。
export function createNotesPort(): NotesPort {
  return {
    ...relationCalls(),
    description: (t) =>
      call(async () => {
        const read =
          t.kind === "item"
            ? await getItemDescription(t.storeId, t.id)
            : await getProjectDescription(t.storeId, t.id);
        return read.text;
      }),
    describe: (t, base, text) =>
      call(async () => {
        const body = { base, text };
        const saved =
          t.kind === "item"
            ? await describeItem(t.storeId, t.id, body)
            : await describeProject(t.storeId, t.id, body);
        return { text: saved.text, conflict: saved.conflict };
      }),
    listComments: (t) =>
      call(async () => {
        const list =
          t.kind === "item"
            ? await listItemComments(t.storeId, t.id)
            : await listProjectComments(t.storeId, t.id);
        return (list ?? []).map(toComment);
      }),
    postComment: (t, body) =>
      call(async () =>
        toComment(
          t.kind === "item"
            ? await postItemComment(t.storeId, t.id, { body })
            : await postProjectComment(t.storeId, t.id, { body }),
        ),
      ),
    editComment: (t, commentId, body) =>
      call(async () =>
        toComment(
          t.kind === "item"
            ? await editItemComment(t.storeId, t.id, commentId, { body })
            : await editProjectComment(t.storeId, t.id, commentId, { body }),
        ),
      ),
    deleteComment: (t, commentId) =>
      call(async () => {
        if (t.kind === "item") await deleteItemComment(t.storeId, t.id, commentId);
        else await deleteProjectComment(t.storeId, t.id, commentId);
      }),
    listWorklogs: (storeId, itemId) =>
      call(async () => ((await listWorklogs(storeId, itemId)) ?? []).map(toWorklog)),
    addWorklog: (storeId, itemId, hours, memo) =>
      call(async () => toWorklog(await addWorklog(storeId, itemId, { hours, memo }))),
    deleteWorklog: (storeId, itemId, worklogId) =>
      call(async () => {
        await deleteWorklog(storeId, itemId, worklogId);
      }),
  };
}
