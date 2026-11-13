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
export {
  formatHours,
  hoursOf,
  type ParsedHours,
  parseHours,
  worklogHeading,
} from "./features/notes/hours";
export { appendMention, mentionTargets } from "./features/notes/mentions";
export {
  createRelations,
  groupRelations,
  type RelationGroup,
  type Relations,
  type RelationsDeps,
  type RelationsState,
} from "./features/notes/relations";
export {
  createWorklogs,
  type Worklogs,
  type WorklogsDeps,
  type WorklogsState,
  worklogDeleteDialog,
  worklogDeletedMessage,
} from "./features/notes/worklogs";
export type { Comment, Worklog } from "./model/notes";
export {
  type RelationCandidate,
  type RelationLabel,
  type RelationRow,
  relationLabels,
} from "./model/relation";
export type { DescribeResult, NotesPort, NoteTarget } from "./port/notes";
