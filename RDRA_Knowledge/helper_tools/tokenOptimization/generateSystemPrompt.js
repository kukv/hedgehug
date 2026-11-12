/**
 * Phase実行前にSystem Promptファイルを動的生成する
 * 
 * 使用方法:
 *   const { generateSystemPrompt } = require('./generateSystemPrompt');
 *   const systemPromptPath = generateSystemPrompt(1); // Phase1用
 */

'use strict';

const fs = require('fs');
const path = require('path');
const { 
    phaseSystemPrompts, 
    phaseSharedFiles,
    specPhaseSystemPrompts,
    specPhaseSharedFiles,
} = require('../settings/rdraConfig');

/**
 * プロジェクトルートを見つける（初期要望.txtがある場所）
 * @param {string} startDir - 検索開始ディレクトリ
 * @returns {string} プロジェクトルートのパス
 */
function findProjectRoot(startDir) {
    let dir = path.resolve(startDir);
    const { root } = path.parse(dir);

    while (true) {
        const marker = path.join(dir, '初期要望.txt');
        if (fs.existsSync(marker)) {
            return dir;
        }
        if (dir === root) {
            throw new Error('初期要望.txt が見つからず、プロジェクトルートを特定できません');
        }
        dir = path.dirname(dir);
    }
}

/**
 * 指定Phaseの共通コンテキストを収集してSystem Promptファイルを生成
 * 
 * @param {number} phaseNumber - Phase番号 (1-4)
 * @returns {string} 生成したSystem Promptファイルの絶対パス
 * @throws {Error} 不正なPhase番号の場合
 */
function generateSystemPrompt(phaseNumber) {
    const sharedFiles = phaseSharedFiles[phaseNumber];
    if (!sharedFiles) {
        throw new Error(`不正なPhase番号: ${phaseNumber}`);
    }

    const projectRoot = findProjectRoot(__dirname);
    
    // System Promptの内容を構築
    const parts = [
        `# Phase${phaseNumber} 共通コンテキスト`,
        '',
        '以下は本Phaseの全タスクで共有する背景知識です。',
        '各タスクではこの情報を前提として、指定された出力を行ってください。',
        '（これらのファイルを個別に読み込む必要はありません）',
        '',
        '---',
        '',
    ];

    let loadedCount = 0;
    let skippedFiles = [];

    for (const file of sharedFiles) {
        const filePath = path.join(projectRoot, file.path);
        
        if (fs.existsSync(filePath)) {
            const content = fs.readFileSync(filePath, 'utf-8').trim();
            
            parts.push(`## ${file.name}`);
            parts.push(`（元ファイル: ${file.path}）`);
            parts.push('');
            parts.push('```');
            parts.push(content);
            parts.push('```');
            parts.push('');
            parts.push('---');
            parts.push('');
            
            loadedCount++;
        } else {
            skippedFiles.push(file.path);
        }
    }

    // 出力先ディレクトリを確保
    const outputRelPath = phaseSystemPrompts[phaseNumber];
    const outputAbsPath = path.join(projectRoot, outputRelPath);
    const outputDir = path.dirname(outputAbsPath);
    
    if (!fs.existsSync(outputDir)) {
        fs.mkdirSync(outputDir, { recursive: true });
        console.log(`ディレクトリ作成: ${outputDir}`);
    }

    // System Promptファイルを書き出し
    const content = parts.join('\n');
    fs.writeFileSync(outputAbsPath, content, 'utf-8');
    
    // 結果を表示
    console.log(`System Prompt生成完了: ${outputRelPath}`);
    console.log(`  - 読み込んだファイル: ${loadedCount}件`);
    if (skippedFiles.length > 0) {
        console.warn(`  - スキップしたファイル: ${skippedFiles.join(', ')}`);
    }

    return outputAbsPath;
}

/**
 * RDRASpec（仕様作成）Phase用のSystem Promptファイルを生成
 * 
 * @param {number} phaseNumber - specPhase番号 (1-2)
 * @returns {string} 生成したSystem Promptファイルの絶対パス
 * @throws {Error} 不正なPhase番号の場合
 */
function generateSpecSystemPrompt(phaseNumber) {
    const sharedFiles = specPhaseSharedFiles[phaseNumber];
    if (!sharedFiles) {
        throw new Error(`不正なspecPhase番号: ${phaseNumber}`);
    }

    const projectRoot = findProjectRoot(__dirname);
    
    // System Promptの内容を構築
    const parts = [
        `# RDRASpec Phase${phaseNumber} 共通コンテキスト`,
        '',
        '以下は本Phaseの全タスクで共有する背景知識です。',
        '各タスクではこの情報を前提として、指定された出力を行ってください。',
        '（これらのファイルを個別に読み込む必要はありません）',
        '',
        '---',
        '',
    ];

    let loadedCount = 0;
    let skippedFiles = [];

    for (const file of sharedFiles) {
        const filePath = path.join(projectRoot, file.path);
        
        if (fs.existsSync(filePath)) {
            const content = fs.readFileSync(filePath, 'utf-8').trim();
            
            parts.push(`## ${file.name}`);
            parts.push(`（元ファイル: ${file.path}）`);
            parts.push('');
            parts.push('```');
            parts.push(content);
            parts.push('```');
            parts.push('');
            parts.push('---');
            parts.push('');
            
            loadedCount++;
        } else {
            skippedFiles.push(file.path);
        }
    }

    // 出力先ディレクトリを確保
    const outputRelPath = specPhaseSystemPrompts[phaseNumber];
    const outputAbsPath = path.join(projectRoot, outputRelPath);
    const outputDir = path.dirname(outputAbsPath);
    
    if (!fs.existsSync(outputDir)) {
        fs.mkdirSync(outputDir, { recursive: true });
        console.log(`ディレクトリ作成: ${outputDir}`);
    }

    // System Promptファイルを書き出し
    const content = parts.join('\n');
    fs.writeFileSync(outputAbsPath, content, 'utf-8');
    
    // 結果を表示
    console.log(`Spec System Prompt生成完了: ${outputRelPath}`);
    console.log(`  - 読み込んだファイル: ${loadedCount}件`);
    if (skippedFiles.length > 0) {
        console.warn(`  - スキップしたファイル: ${skippedFiles.join(', ')}`);
    }

    return outputAbsPath;
}

/**
 * 全Phaseの共通ファイル一覧を表示（デバッグ用）
 */
function showPhaseSharedFiles() {
    console.log('=== RDRAZeroOne Phase別 共通ファイル一覧 ===\n');
    
    for (const [phase, files] of Object.entries(phaseSharedFiles)) {
        console.log(`Phase${phase}:`);
        for (const file of files) {
            console.log(`  - ${file.name}: ${file.path}`);
        }
        console.log('');
    }
    
    console.log('=== RDRASpec Phase別 共通ファイル一覧 ===\n');
    
    for (const [phase, files] of Object.entries(specPhaseSharedFiles)) {
        console.log(`specPhase${phase}:`);
        for (const file of files) {
            console.log(`  - ${file.name}: ${file.path}`);
        }
        console.log('');
    }
}

// CLIとして実行された場合
if (require.main === module) {
    const args = process.argv.slice(2);
    
    if (args.length === 0 || args[0] === '--help' || args[0] === '-h') {
        console.log(`
System Prompt生成ツール

使い方:
  node generateSystemPrompt.js <phase>        指定Phaseの System Promptを生成 (1-4)
  node generateSystemPrompt.js spec <phase>   RDRASpec用の System Promptを生成 (1-2)
  node generateSystemPrompt.js --list         Phase別共通ファイル一覧を表示
  node generateSystemPrompt.js --all          全PhaseのSystem Promptを生成

例:
  node generateSystemPrompt.js 1              RDRAZeroOne Phase1用を生成
  node generateSystemPrompt.js spec 1         RDRASpec Phase1用を生成
  node generateSystemPrompt.js --all          全Phase用を生成（RDRAZeroOne + RDRASpec）
`);
        process.exit(0);
    }
    
    if (args[0] === '--list') {
        showPhaseSharedFiles();
        process.exit(0);
    }
    
    if (args[0] === '--all') {
        console.log('=== RDRAZeroOne Phase ===');
        for (const phase of [1, 2, 3, 4]) {
            try {
                generateSystemPrompt(phase);
            } catch (err) {
                console.error(`Phase${phase} エラー: ${err.message}`);
            }
        }
        console.log('\n=== RDRASpec Phase ===');
        for (const phase of [1, 2]) {
            try {
                generateSpecSystemPrompt(phase);
            } catch (err) {
                console.error(`specPhase${phase} エラー: ${err.message}`);
            }
        }
        process.exit(0);
    }
    
    // spec <phase> の場合
    if (args[0] === 'spec') {
        const phaseNumber = parseInt(args[1], 10);
        if (isNaN(phaseNumber) || phaseNumber < 1 || phaseNumber > 2) {
            console.error('エラー: specPhase番号は 1-2 の整数で指定してください');
            process.exit(1);
        }
        
        try {
            generateSpecSystemPrompt(phaseNumber);
        } catch (err) {
            console.error(`エラー: ${err.message}`);
            process.exit(1);
        }
        process.exit(0);
    }
    
    // <phase> の場合（RDRAZeroOne）
    const phaseNumber = parseInt(args[0], 10);
    if (isNaN(phaseNumber) || phaseNumber < 1 || phaseNumber > 4) {
        console.error('エラー: Phase番号は 1-4 の整数で指定してください');
        process.exit(1);
    }
    
    try {
        generateSystemPrompt(phaseNumber);
    } catch (err) {
        console.error(`エラー: ${err.message}`);
        process.exit(1);
    }
}

module.exports = { 
    generateSystemPrompt,
    generateSpecSystemPrompt,
    showPhaseSharedFiles,
    findProjectRoot,
};
