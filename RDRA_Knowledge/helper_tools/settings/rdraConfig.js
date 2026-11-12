'use strict';

/**
 * Phaseのプロンプトファイル（並列実行用）　実行のプロンプトファイル
 */
const phase1PromptMap = [
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/アクター生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/外部システム生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/ビジネスポリシー生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/ビジネスパラメータ生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/業務生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/情報生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/状態生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/バリエーション生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/条件生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/要求生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase1/初期要望分析生成.md'}
];

const phase2PromptMap = [
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase2/業務生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase2/情報生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase2/状態生成.md' }
];

const phase3PromptMap = [
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase3/アクター生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase3/BUC生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase3/情報生成.md'}
];

const phase4PromptMap = [
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase4/BUC生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase4/バリエーション生成.md'},
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase4/条件生成.md' },
  { prompt: 'RDRA_Knowledge/0_RDRAZeroOne/phase4/状態生成.md'}
];

/**
 * RDRAZeroOne　Phase1~Phase4 RDRA RDRASpecのファイル
 */
const phase1Files = [
  'アクター.tsv',
  '外部システム.tsv',
  'ビジネスポリシー.tsv',
  'ビジネスパラメータ.tsv',
  '業務.tsv',
  '情報.tsv',
  '状態.tsv',
  'バリエーション.tsv',
  '条件.tsv',
  '初期要望分析.md'
];

const phase2Files = [
  '業務.tsv',
  '情報.tsv',
  '状態.tsv',
];

const phase3Files = [
  'アクター.tsv',
  'BUC.tsv',
  '情報.tsv'
];

const phase4Files = [
  'BUC.tsv',
  '条件.tsv',
  'バリエーション.tsv',
  '状態.tsv'
];

const rdraFiles = [
  'システム概要.json',
  'BUC.tsv',
  'アクター.tsv',
  '外部システム.tsv',
  '情報.tsv',
  '状態.tsv',
  '条件.tsv',
  'バリエーション.tsv'
];

// RDRASpec（仕様）成果物ファイル
// NOTE: 画面定義は「画面照会.json」を正とする（過去互換で ui.json を扱う箇所は menuAction 側で分岐）
const specFiles = [
  '論理データモデル.md',
  'ビジネスルール.md',
  '画面照会.json'
];

const specPhase1Files = [
  '21_論理データ生成.md',
  '22_ビジネスルール生成.md',
  '23_画面一覧生成.md'
];
const specPhase2Files = [
  '24_画面照会生成.md'
];

/**
 * 仕様作成用のプロンプトマップ（並列実行用）
 */
const specPhase1PromptMap = [
  { prompt: 'RDRA_Knowledge/2_RDRASpec/phase1/21_論理データ生成.md' },
  { prompt: 'RDRA_Knowledge/2_RDRASpec/phase1/22_ビジネスルール生成.md' },
  { prompt: 'RDRA_Knowledge/2_RDRASpec/phase1/23_画面一覧生成.md' },
  { prompt: 'RDRA_Knowledge/2_RDRASpec/phase1/24_BUC画面生成.md' },
  { prompt: 'RDRA_Knowledge/2_RDRASpec/phase1/25_アクター画面生成.md' },
];
const specPhase2PromptMap = [
  { prompt: 'RDRA_Knowledge/2_RDRASpec/phase2/26_画面照会生成.md' },
];

/**
 * Phase別のSystem Promptファイルパス
 * generateSystemPrompt.js で生成される
 */
const phaseSystemPrompts = {
  1: 'RDRA_Knowledge/system_prompts/phase1_system.md',
  2: 'RDRA_Knowledge/system_prompts/phase2_system.md',
  3: 'RDRA_Knowledge/system_prompts/phase3_system.md',
  4: 'RDRA_Knowledge/system_prompts/phase4_system.md',
};

/**
 * RDRASpec（仕様作成）Phase別のSystem Promptファイルパス
 */
const specPhaseSystemPrompts = {
  1: 'RDRA_Knowledge/system_prompts/spec_phase1_system.md',
  2: 'RDRA_Knowledge/system_prompts/spec_phase2_system.md',
};

/**
 * Phase別の共通読み込みファイル定義
 * System Prompt生成時にこれらのファイル内容を結合する
 * 
 * 各Phaseで並列実行される全タスクが読む必要のあるファイルをここに列挙
 */
const phaseSharedFiles = {
  // Phase1: 初期要望から各要素を洗い出す
  // 全タスクが「初期要望.txt」と「RDRA.md」を参照する
  1: [
    { name: 'RDRAナレッジ', path: 'RDRA_Knowledge/1_RDRA/RDRA.md' },
    { name: '初期要望', path: '初期要望.txt' },
  ],
  
  // Phase2: Phase1の成果物を基に詳細化
  // 全タスクがPhase1の複数ファイルを参照する
  2: [
    { name: 'RDRAナレッジ', path: 'RDRA_Knowledge/1_RDRA/RDRA.md' },
    { name: '初期要望分析', path: '0_RDRAZeroOne/phase1/初期要望分析.md' },
    { name: '業務（Phase1）', path: '0_RDRAZeroOne/phase1/業務.tsv' },
    { name: 'アクター（Phase1）', path: '0_RDRAZeroOne/phase1/アクター.tsv' },
    { name: '状態（Phase1）', path: '0_RDRAZeroOne/phase1/状態.tsv' },
    { name: 'ビジネスポリシー', path: '0_RDRAZeroOne/phase1/ビジネスポリシー.tsv' },
  ],
  
  // Phase3: Phase2の成果物を基にBUCと情報を整理
  3: [
    { name: 'RDRAナレッジ', path: 'RDRA_Knowledge/1_RDRA/RDRA.md' },
    { name: '業務（Phase2）', path: '0_RDRAZeroOne/phase2/業務.tsv' },
    { name: 'アクター（Phase1）', path: '0_RDRAZeroOne/phase1/アクター.tsv' },
    { name: '情報（Phase2）', path: '0_RDRAZeroOne/phase2/情報.tsv' },
  ],
  
  // Phase4: Phase3の成果物を基に最終整理
  4: [
    { name: 'RDRAナレッジ', path: 'RDRA_Knowledge/1_RDRA/RDRA.md' },
    { name: 'BUC（Phase3）', path: '0_RDRAZeroOne/phase3/BUC.tsv' },
    { name: '情報（Phase3）', path: '0_RDRAZeroOne/phase3/情報.tsv' },
    { name: '状態（Phase2）', path: '0_RDRAZeroOne/phase2/状態.tsv' },
  ],
};

/**
 * RDRASpec（仕様作成）Phase別の共通読み込みファイル定義
 * specPhase1: 論理データ/ビジネスルール/画面一覧/BUC画面/アクター画面（5タスク並列）
 * specPhase2: 画面照会（1タスク、キャッシュ効果は限定的）
 */
const specPhaseSharedFiles = {
  // specPhase1: 全5タスクが共通で読み込むファイル
  1: [
    { name: 'RDRAナレッジ', path: 'RDRA_Knowledge/1_RDRA/RDRA.md' },
    { name: 'RDRAGraph', path: 'RDRA_Knowledge/1_RDRA/RDRAGraph.md' },
    { name: '関連データ', path: '1_RDRA/関連データ.txt' },
  ],
  
  // specPhase2: 画面照会生成（1タスクのみだがSystem Promptとして整理）
  2: [
    { name: 'RDRAナレッジ', path: 'RDRA_Knowledge/1_RDRA/RDRA.md' },
    { name: 'RDRAGraph', path: 'RDRA_Knowledge/1_RDRA/RDRAGraph.md' },
    { name: '関連データ', path: '1_RDRA/関連データ.txt' },
    { name: '論理データモデル', path: '2_RDRASpec/論理データモデル.md' },
    { name: 'BUC画面', path: '2_RDRASpec/phase1/BUC画面.json' },
    { name: 'アクター画面', path: '2_RDRASpec/phase1/アクター画面.json' },
    { name: '画面一覧', path: '2_RDRASpec/phase1/画面一覧.json' },
  ],
};

module.exports = {
  phase1PromptMap,
  phase2PromptMap,
  phase3PromptMap,
  phase4PromptMap,
  specPhase1PromptMap,
  specPhase2PromptMap,
  phase1Files,
  phase2Files,
  phase3Files,
  phase4Files,
  rdraFiles,
  specFiles,
  specPhase1Files,
  specPhase2Files,
  phaseSystemPrompts,
  phaseSharedFiles,
  specPhaseSystemPrompts,
  specPhaseSharedFiles,
};


