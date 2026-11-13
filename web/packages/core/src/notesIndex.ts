// 詳細パネルの説明・Markdown・コメントなど(トラック A4a)の core の export。index.ts が全部を再公開する
// (index.ts は 300 行の上限に近いので、A4a の分はここに足す)。

export {
  createDrafts,
  type DiscardPrompt,
  type Drafts,
  type LeaveGuard,
} from "./features/drafts/drafts";
export {
  type Block as MarkdownBlock,
  type Inline as MarkdownInline,
  isMemberOf,
  tokenize,
} from "./features/markdown/markdown";
export {
  type Comments,
  type CommentsDeps,
  type CommentsState,
  commentAvatar,
  commentDeleteDialog,
  commentDeletedMessage,
  commentPlaceholder,
  createComments,
  isMine,
} from "./features/notes/comments";
export {
  createDescriptionEditor,
  type DescriptionDeps,
  type DescriptionEditor,
  type DescriptionState,
  descriptionConflictMessage,
} from "./features/notes/description";
export { appendMention, mentionTargets } from "./features/notes/mentions";
export type { Comment } from "./model/notes";
export type { DescribeResult, NotesPort, NoteTarget } from "./port/notes";
