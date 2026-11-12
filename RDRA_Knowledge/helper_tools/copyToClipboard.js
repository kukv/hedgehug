const fs = require('fs');
const path = require('path');
const { exec } = require('child_process');
const os = require('os');

function copyToClipboard(content) {
    // 空の内容の場合はコピーを実行しない
    if (!content || content.trim() === '') {
        console.log('Warning: コンテンツが空のため、クリップボードにコピーしませんでした');
        return;
    }

    if (process.platform === 'win32') {
        // Windows: UTF-8の一時ファイル経由でPowerShell → 失敗時はclip
        const tempFilePath = path.join(os.tmpdir(), 'rdra_clipboard.txt');
        try {
            fs.writeFileSync(tempFilePath, content, { encoding: 'utf8' });
        } catch (_) {}

        const psCommand = `powershell.exe -NoProfile -Command "Get-Content -LiteralPath '${tempFilePath.replace(/'/g, "''")}' -Raw -Encoding UTF8 | Set-Clipboard"`;
        exec(psCommand, (error) => {
            try { fs.unlinkSync(tempFilePath); } catch (_) {}

            if (error) {
                const clipProcess = exec('clip', (clipErr) => {
                    if (clipErr) {
                        console.log('Warning: クリップボードへのコピーに失敗しました');
                    } else {
                        console.log('クリップボードにコピーしました (fallback: clip)');
                    }
                });

                if (clipProcess.stdin) {
                    clipProcess.stdin.write(content, 'utf8');
                    clipProcess.stdin.end();
                }
            } else {
                console.log('クリップボードにコピーしました');
            }
        });
    } else if (process.platform === 'darwin') {
        // macOS: pbcopy にパイプ
        const pb = exec('pbcopy', (err) => {
            if (err) {
                console.log('Warning: クリップボードへのコピーに失敗しました (pbcopy)');
            } else {
                console.log('クリップボードにコピーしました');
            }
        });
        if (pb.stdin) {
            pb.stdin.write(content, 'utf8');
            pb.stdin.end();
        }
    } else {
        console.log('Warning: 未対応のプラットフォームです (Windows/macOS をサポート)');
    }
}

// メイン処理：コマンドライン引数からファイルパスを取得
function main() {
    const args = process.argv.slice(2);
    
    // デフォルトファイルパスの定義（日本語ファイル名の文字化け対策）
    const defaultFiles = {
        'graph': '1_RDRA/関連データ.txt',
        'zeroone': '1_RDRA/ZeroOne.txt'
    };
    
    let filePath;
    
    if (args.length === 0) {
        // 引数なしの場合はグラフデータをデフォルトに
        filePath = defaultFiles['graph'];
        console.log('デフォルトファイルを使用: 1_RDRA/関連データ.txt');
    } else if (defaultFiles[args[0].toLowerCase()]) {
        // キーワード指定の場合
        filePath = defaultFiles[args[0].toLowerCase()];
    } else {
        filePath = args[0];
    }
    
    // ファイルパスを解決（相対パスの場合は絶対パスに変換）
    const resolvedPath = path.resolve(filePath);
    
    if (!fs.existsSync(resolvedPath)) {
        console.log(`Error: ファイルが見つかりません: ${resolvedPath}`);
        process.exit(1);
    }
    
    try {
        const content = fs.readFileSync(resolvedPath, 'utf-8');
        console.log(`ファイルを読み込みました: ${resolvedPath}`);
        copyToClipboard(content);
    } catch (error) {
        console.log(`Error: ファイルの読み込みに失敗しました: ${error.message}`);
        process.exit(1);
    }
}

main();

