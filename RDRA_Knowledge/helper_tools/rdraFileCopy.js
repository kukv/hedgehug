const fs = require('fs');
const path = require('path');

const {
  phase1Files,
  phase2Files,
  phase3Files,
  phase4Files,
  rdraFiles,
} = require('./settings/rdraConfig');

/**
 * RDRAファイルコピープログラム
 * - `menu.js` と同じ成果物定義（phase1Files〜phase4Files / rdraFiles）を参照して判定する
 * - Phase1〜Phase4の成果物が揃っていることを確認してから、`1_RDRA` に必要ファイルをコピーする
 */

const phaseConfigs = [
  { phase: 'phase1', sourceDir: '0_RDRAZeroOne/phase1', files: phase1Files },
  { phase: 'phase2', sourceDir: '0_RDRAZeroOne/phase2', files: phase2Files },
  { phase: 'phase3', sourceDir: '0_RDRAZeroOne/phase3', files: phase3Files },
  { phase: 'phase4', sourceDir: '0_RDRAZeroOne/phase4', files: phase4Files },
];

// コピー元の優先順位（新しいPhaseを優先）
const sourcePriority = ['phase4', 'phase3', 'phase2', 'phase1'];

const TARGET_DIR = '1_RDRA';
const EXCLUDE_FROM_COPY = new Set(['システム概要.json']); // Phase5で生成されるため

function checkFileExists(filePath) {
  return fs.existsSync(filePath);
}

function copyFile(source, target) {
  try {
    fs.copyFileSync(source, target);
    console.log(`✓ コピー完了: ${path.basename(source)}`);
    return true;
  } catch (error) {
    console.error(`✗ コピー失敗: ${path.basename(source)} - ${error.message}`);
    return false;
  }
}

function ensureTargetDirectory(targetDir) {
  if (!fs.existsSync(targetDir)) {
    fs.mkdirSync(targetDir, { recursive: true });
    console.log(`対象フォルダーを作成しました: ${targetDir}`);
  }
}

function validatePhaseOutputs() {
  console.log('=== ファイル存在チェック ===');

  let allFilesExist = true;
  const missingFiles = [];

  for (const cfg of phaseConfigs) {
    console.log(`\n${cfg.phase}フォルダーのチェック:`);

    for (const fileName of cfg.files) {
      const filePath = path.join(cfg.sourceDir, fileName);
      if (checkFileExists(filePath)) {
        console.log(`  ✓ ${fileName} - 存在`);
      } else {
        console.log(`  ✗ ${fileName} - 存在しない`);
        allFilesExist = false;
        missingFiles.push(`${cfg.sourceDir}/${fileName}`);
      }
    }
  }

  if (!allFilesExist) {
    console.log('\n⚠️  以下のファイルが見つかりません:');
    missingFiles.forEach(file => console.log(`  - ${file}`));
    console.log('\n必要なファイルが揃っていないため、コピーを実行しません。');
    return false;
  }

  return true;
}

function buildCopyPlan() {
  const phaseMap = new Map(phaseConfigs.map(cfg => [cfg.phase, cfg]));
  const filesToCopy = rdraFiles.filter(f => !EXCLUDE_FROM_COPY.has(f));

  const plan = [];
  for (const fileName of filesToCopy) {
    const sourcePhase = sourcePriority.find(phase => phaseMap.get(phase)?.files?.includes(fileName));
    if (!sourcePhase) {
      throw new Error(`コピー元フェーズが特定できません: ${fileName}（rdraFiles と phase*Files の整合性を確認してください）`);
    }
    const cfg = phaseMap.get(sourcePhase);
    plan.push({
      fileName,
      sourceDir: cfg.sourceDir,
      sourcePhase,
      targetDir: TARGET_DIR,
    });
  }
  return plan;
}

function main() {
  console.log('=== RDRAファイルコピープログラム ===\n');

  let plan;
  try {
    plan = buildCopyPlan();
  } catch (e) {
    console.error(`✗ コピー計画の作成に失敗しました: ${e.message}`);
    process.exit(1);
  }

  console.log('=== コピー計画（優先: phase4 > phase3 > phase2 > phase1） ===');
  plan.forEach(p => {
    console.log(`- ${p.fileName}: ${p.sourcePhase} -> ${p.targetDir}`);
  });

  // ファイル存在チェック
  if (!validatePhaseOutputs()) {
    process.exit(1);
  }

  console.log('\n=== ファイルコピー開始 ===');
  ensureTargetDirectory(TARGET_DIR);

  let successCount = 0;
  let totalFiles = 0;

  for (const item of plan) {
    const sourcePath = path.join(item.sourceDir, item.fileName);
    const targetPath = path.join(item.targetDir, item.fileName);

    totalFiles++;

    if (checkFileExists(targetPath)) {
      console.log(`  上書き対象: ${item.fileName}`);
    }

    if (copyFile(sourcePath, targetPath)) {
      successCount++;
    }
  }

  console.log('\n=== コピー結果 ===');
  console.log(`成功: ${successCount}/${totalFiles} ファイル`);

  console.log('\n=== 最終確認 ===');
  for (const item of plan) {
    const targetPath = path.join(TARGET_DIR, item.fileName);
    if (checkFileExists(targetPath)) {
      console.log(`✓ ${item.fileName} - 配置確認`);
    } else {
      console.log(`✗ ${item.fileName} - 配置未確認`);
    }
  }
}

if (require.main === module) {
  main();
}

module.exports = { main, buildCopyPlan };