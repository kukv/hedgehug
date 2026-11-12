const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = 3002;
const HOST = 'localhost';

// 入力JSON（優先: 画面照会.json、フォールバック: ui.json）
const SPEC_DIR = path.join(__dirname, '../../../2_RDRASpec');
const SCREEN_INQUIRY_JSON_PATH = path.join(SPEC_DIR, '画面照会.json');
const UI_JSON_FALLBACK_PATH = path.join(SPEC_DIR, 'ui.json');

function resolveUiJsonPath() {
  if (fs.existsSync(SCREEN_INQUIRY_JSON_PATH)) return SCREEN_INQUIRY_JSON_PATH;
  if (fs.existsSync(UI_JSON_FALLBACK_PATH)) return UI_JSON_FALLBACK_PATH;
  return null;
}

const mimeTypes = {
  '.html': 'text/html',
  '.json': 'application/json',
  '.css': 'text/css',
  '.js': 'text/javascript',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
};

const server = http.createServer((req, res) => {
  // シャットダウン（クライアントの「閉じる」ボタン用）
  if (req.url === '/shutdown') {
    res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' });
    res.end(JSON.stringify({ message: 'サーバーをシャットダウンします' }), 'utf-8');
    console.log('シャットダウンリクエストを受信しました。サーバーを終了します...');
    setTimeout(() => {
      server.close(() => {
        console.log('サーバーが正常に終了しました。');
        process.exit(0);
      });
    }, 100);
    return;
  }

  // 画面照会JSON
  if (req.url === '/ui.json') {
    const uiJsonPath = resolveUiJsonPath();
    if (!uiJsonPath) {
      res.writeHead(404, { 'Content-Type': 'application/json; charset=utf-8' });
      res.end(
        JSON.stringify(
          {
            error: 'UI JSON not found',
            message:
              '2_RDRASpec/画面照会.json（または ui.json）が存在しません。先にメニューで仕様ファイルを作成してください。',
          },
          null,
          2
        ),
        'utf-8'
      );
      return;
    }

    fs.readFile(uiJsonPath, (error, content) => {
      if (error) {
        res.writeHead(500, { 'Content-Type': 'application/json; charset=utf-8' });
        res.end(JSON.stringify({ error: error.code || 'READ_ERROR' }), 'utf-8');
        return;
      }
      res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' });
      res.end(content, 'utf-8');
    });
    return;
  }

  // 静的配信
  let filePath;
  if (req.url === '/' || req.url === '/index.html') {
    filePath = path.join(__dirname, 'bucActorUI.html');
  } else {
    // 安全のため web_tool 配下のみ配信（パストラバーサル対策）
    const reqPath = decodeURIComponent(req.url);
    const normalized = path.normalize(reqPath).replace(/^(\.\.[/\\])+/, '');
    filePath = path.join(__dirname, normalized);
  }

  const extname = String(path.extname(filePath)).toLowerCase();
  const contentType = mimeTypes[extname] || 'application/octet-stream';

  fs.readFile(filePath, (error, content) => {
    if (error) {
      if (error.code === 'ENOENT') {
        res.writeHead(404, { 'Content-Type': 'text/html; charset=utf-8' });
        res.end('<h1>404 Not Found</h1>', 'utf-8');
      } else {
        res.writeHead(500, { 'Content-Type': 'text/plain; charset=utf-8' });
        res.end(`Server Error: ${error.code}`, 'utf-8');
      }
      return;
    }
    res.writeHead(200, { 'Content-Type': `${contentType}; charset=utf-8` });
    res.end(content, 'utf-8');
  });
});

server.listen(PORT, HOST, () => {
  console.log(`簡易HTTPサーバーが起動しました: http://${HOST}:${PORT}/`);
  console.log('サーバーを停止するには Ctrl+C を押してください');
});

server.on('error', (error) => {
  if (error.code === 'EADDRINUSE') {
    console.error(`エラー: ポート ${PORT} は既に使用されています。`);
    console.error('別のプロセスを終了してから再度実行してください。');
  } else {
    console.error('サーバーエラー:', error);
  }
  process.exit(1);
});

process.on('SIGINT', () => {
  console.log('\nサーバーを終了しています...');
  server.close(() => {
    console.log('サーバーが終了しました。');
    process.exit(0);
  });
});

