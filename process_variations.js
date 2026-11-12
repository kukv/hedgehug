const fs = require('fs');

// phase1のバリエーション.tsvを読込む
const phase1Content = fs.readFileSync('0_RDRAZeroOne/phase1/バリエーション.tsv', 'utf8');
const phase3Content = fs.readFileSync('0_RDRAZeroOne/phase3/情報.tsv', 'utf8');

// phase1のバリエーションをパース
const lines = phase1Content.trim().split('\n');
const phase1Variations = [];

// ヘッダーをスキップして処理
for (let i = 1; i < lines.length; i++) {
  const cells = lines[i].split('\t');
  if (cells.length >= 2) {
    phase1Variations.push({
      variation: cells[0],
      values: cells[1],
      description: cells[2] || ''
    });
  }
}

// phase3の情報をパース
const phase3Lines = phase3Content.trim().split('\n');
const contextMap = {};

for (let i = 1; i < phase3Lines.length; i++) {
  const cells = phase3Lines[i].split('\t');
  if (cells.length >= 6) {
    const context = cells[0];
    const info = cells[1];
    const variationsFromInfo = cells[5]; // バリエーション列

    if (variationsFromInfo) {
      // バリエーション列に複数のバリエーションがある場合
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

// phase4のバリエーション.tsvを作成
const phase4Variations = [];

for (const v of phase1Variations) {
  // コンテキストを決定
  let context = contextMap[v.variation];
  
  if (!context) {
    // コンテキストがない場合は、バリエーション名から推測
    // バリエーション名の前の部分をコンテキストとして使用
    if (v.variation.includes('スタッフ')) {
      context = 'スタッフ管理';
    } else if (v.variation.includes('介護会員')) {
      context = '会員管理';
    } else if (v.variation.includes('施設')) {
      context = '施設管理';
    } else if (v.variation.includes('訪問')) {
      context = '訪問計画管理';
    } else if (v.variation.includes('費用') || v.variation.includes('請求')) {
      context = '費用管理';
    } else if (v.variation.includes('サービス')) {
      context = '実績管理';
    } else {
      // 汎用コンテキストを推測
      const words = v.variation.split(/[の・]/);
      context = words[0] || 'その他';
    }
  }

  phase4Variations.push({
    context: context,
    variation: v.variation,
    values: v.values,
    description: v.description
  });
}

// ファイル出力
const outputDir = '0_RDRAZeroOne/phase4';
if (!fs.existsSync(outputDir)) {
  fs.mkdirSync(outputDir, { recursive: true });
}

let tsvContent = 'コンテキスト\tバリエーション\t値\t説明\n';
for (const v of phase4Variations) {
  tsvContent += `${v.context}\t${v.variation}\t${v.values}\t${v.description}\n`;
}

fs.writeFileSync(`${outputDir}/バリエーション.tsv`, tsvContent, 'utf8');
console.log('Phase4 variations created successfully');
