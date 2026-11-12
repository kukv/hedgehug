/**
 * 並行実行ランナー（シンプル版）
 * 
 * 使用方法:
 *   node parallel-runner.js file1.txt file2.txt file3.txt
 *   node parallel-runner.js {prompt/run1.md,output/run1.tsv} {prompt/run2.md,output/run2.tsv}   # output 側は無視されます（互換用）
 */

const { runAIWithPrefix, getResolvedDefaultProvider } = require('./node-file-runner-V3');
const fs = require('fs');
const path = require('path');

// ========================================
// ヘルプ表示
// ========================================
function showHelp() {
    console.log(`
並行実行ランナー（シンプル版）

使い方:
  node parallel-runner.js <files...> [options]
  node parallel-runner.js {input,output} {input,output} ... [options]   ※ output 側は無視（互換用）

ファイル指定形式:
  file.txt                    入力のみ
  {input.md,output.tsv}       互換用（output 側は無視されます）

オプション:
  --timeout <ms>              タイムアウト（ミリ秒、デフォルト: 180000）
  --system-prompt <path>      System Promptファイルパス（全タスク共通）
  --help, -h                  このヘルプを表示

例:
  node parallel-runner.js file1.txt file2.txt file3.txt
  node parallel-runner.js file1.txt file2.txt --system-prompt shared.md
  node parallel-runner.js {prompt/run1.md,output/run1.tsv} {prompt/run2.md,output/run2.tsv}   # output 側は無視（互換用）

モデル/プロバイダー設定:
  モデル/プロバイダーおよび各種オプションは、プロジェクトルート（初期要望.txtと同じ階層）の
  「モデル設定.json」から読み込みます（CLIからは指定しません）。
  
System Prompt:
  --system-prompt で指定されたファイルは、全並列タスクの共通コンテキストとして使用されます。
  Claude: --system-prompt オプションで渡されます
  Gemini: --system-instruction オプションで渡されます
  共通コンテキストがキャッシュされることで、トークン消費を大幅に削減できます。
`);
}

// ========================================
// 入力ファイルをパース
// ========================================
function parseFilePair(arg) {
    // {input,output} 形式が来ても input 側のみを取り出す（互換用）
    const match = arg.match(/^\{([^,]+),?.*\}$/);
    return {
        input: match ? match[1].trim() : arg.trim(),
    };
}

// ========================================
// コマンドライン引数解析
// ========================================
function parseArgs(argv) {
    const args = argv.slice(2);
    const config = {
        filePairs: [],  // { input } の配列（{input,output} 形式は互換用で input のみ使用）
        options: {
            timeout: 180000, // 3分
            systemPrompt: null,  // System Promptファイルパス
        },
    };

    for (let i = 0; i < args.length; i++) {
        const arg = args[i];

        if (arg === '--help' || arg === '-h') {
            showHelp();
            process.exit(0);
        } else if (arg === '--timeout') {
            config.options.timeout = parseInt(args[++i], 10);
        } else if (arg === '--system-prompt') {
            config.options.systemPrompt = args[++i];
        } else if (!arg.startsWith('-')) {
            config.filePairs.push(parseFilePair(arg));
        }
    }

    return config;
}

// ========================================
// ファイルからプロンプトを読み込み
// ========================================
function readPrompt(filePath) {
    const absolutePath = path.resolve(filePath);
    return fs.readFileSync(absolutePath, 'utf-8').trim();
}

// ========================================
// 単一のプロンプトを実行
// ========================================
async function executePrompt(filePair, options = {}) {
    const { input: inputPath } = filePair;
    const startTime = Date.now();
    const baseName = path.basename(inputPath, path.extname(inputPath));

    try {
        const prompt = readPrompt(inputPath);
        
        // プレフィックスを設定（ファイル名をベースに）
        const execOptions = {
            ...options,
            prefix: baseName,
        };
        
        // プレフィックス付きリアルタイム出力モードで実行
        await runAIWithPrefix(prompt, execOptions);
        
        const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
        console.log(`[${baseName}] ✅ OK (${elapsed}s)`);
        
        return {
            input: inputPath,
            success: true,
            elapsed: elapsed,
        };
    } catch (error) {
        const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
        console.error(`[${baseName}] ❌ NG (${elapsed}s) ${error.message}`);
        
        return {
            input: inputPath,
            success: false,
            error: error.message,
            elapsed: elapsed,
        };
    }
}

// ========================================
// 並行実行
// ========================================
async function executeParallel(filePairs, options = {}) {
    if (filePairs.length === 0) {
        console.log('\n⏭️ 実行するファイルがありません');
        return [];
    }
    
    console.log('\n🚀 並行実行開始');
    const inputFiles = filePairs.map(fp => fp.input).join(', ');
    console.log(`対象ファイル: ${inputFiles}`);
    
    // System Prompt情報を表示
    if (options.systemPrompt) {
        console.log(`System Prompt: ${options.systemPrompt}`);
    }
    
    const startTime = Date.now();
    
    const results = await Promise.all(
        filePairs.map(filePair => executePrompt(filePair, options))
    );
    
    const totalElapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    console.log(`\n🏁 並行実行完了 (合計: ${totalElapsed}秒)`);
    
    return results;
}
// ========================================
// メイン実行
// ========================================
async function main() {
    const config = parseArgs(process.argv);
    
    // ファイルが指定されていない場合
    if (config.filePairs.length === 0) {
        console.error('エラー: 実行するファイルが指定されていません（--help を参照）');
        process.exit(1);
    }
    
    // 設定表示
    console.log(`files: ${config.filePairs.length}, provider(from config): ${getResolvedDefaultProvider()}, timeout: ${config.options.timeout}ms`);
    
    const totalStartTime = Date.now();
    
    try {
        // 並行実行
        const results = await executeParallel(config.filePairs, config.options);

        // 方式A: 全件実行してから集計し、失敗が1つでもあれば exit code を非0にする
        const failed = results.filter(r => !r.success);
        if (failed.length > 0) {
            console.error(`\n❌ 失敗: ${failed.length}/${results.length}`);
            failed.forEach(r => {
                console.error(`- ${r.input}: ${r.error || 'unknown error'}`);
            });
            process.exitCode = 1;
            return;
        }
    } catch (error) {
        console.error('fatal:', error.message);
        process.exit(1);
    }
}

if (require.main === module) {
    main().catch(console.error);
}
