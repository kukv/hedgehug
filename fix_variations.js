const fs = require('fs');

// phase3の情報をパース
const phase3Content = fs.readFileSync('0_RDRAZeroOne/phase3/情報.tsv', 'utf8');
const phase3Lines = phase3Content.trim().split('\n');

// バリエーション名とコンテキストのマッピングを作成
const contextMap = {};
for (let i = 1; i < phase3Lines.length; i++) {
  const cells = phase3Lines[i].split('\t');
  if (cells.length >= 6) {
    const context = cells[0];
    const variationsFromInfo = cells[5]; // バリエーション列

    if (variationsFromInfo) {
      const varList = variationsFromInfo.split('、');
      for (const v of varList) {
        const trimmedVar = v.trim();
        if (!contextMap[trimmedVar]) {
          contextMap[trimmedVar] = context;
        }
      }
    }
  }
}

// phase4のバリエーション.tsvを修正
const content = fs.readFileSync('0_RDRAZeroOne/phase4/バリエーション.tsv', 'utf8');
const lines = content.trim().split('\n');
const output = [];

// ヘッダー行
output.push(lines[0]);

// データ行
for (let i = 1; i < lines.length; i++) {
  const cells = lines[i].split('\t');
  const variation = cells[1];
  
  // コンテキストを修正
  let context = contextMap[variation];
  if (!context) {
    // フォールバック処理
    if (variation.includes('スタッフ') || variation.includes('勤務')) {
      context = 'スタッフ管理';
    } else if (variation.includes('介護会員')) {
      context = '会員管理';
    } else if (variation.includes('施設')) {
      context = '施設管理';
    } else if (variation.includes('訪問')) {
      context = '訪問計画管理';
    } else if (variation.includes('費用') || variation.includes('請求')) {
      context = '費用管理';
    } else if (variation.includes('サービス')) {
      context = '実績管理';
    } else {
      context = cells[0]; // 元のコンテキストを使用
    }
  }
  
  cells[0] = context;
  output.push(cells.join('\t'));
}

fs.writeFileSync('0_RDRAZeroOne/phase4/バリエーション.tsv', output.join('\n'), 'utf8');
console.log('Variations fixed successfully');
