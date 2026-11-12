const fs = require('fs');
const path = require('path');

const rdraDir = path.resolve(__dirname, '..', '..', '1_RDRA');
const outputFile = path.join(rdraDir, 'ZeroOne.txt');

function readSystemName() {
    const systemOverviewPath = path.join(rdraDir, 'システム概要.json');
    if (!fs.existsSync(systemOverviewPath)) return '';
    try {
        const systemOverview = JSON.parse(fs.readFileSync(systemOverviewPath, 'utf-8'));
        return '\t' + systemOverview.system_name || '';
    } catch (error) {
        console.log('Warning: システム概要.json の読み込みに失敗しました');
        return '';
    }
}

function readTsvLines(filePath) {
    const content = fs.readFileSync(filePath, 'utf-8');
    return content
        .replace(/\r\n/g, '\n')
        .split('\n')
        .map(line => line.trimEnd())
        .filter(line => line.trim() !== '');
}

function splitTsv(line) {
    return line.split('\t');
}

function joinNonEmpty(parts, sep = ' / ') {
    return parts.filter(p => (p ?? '').toString().trim() !== '').join(sep);
}

function emitSection({ title, fileName, outHeader, mapRow }) {
    const filePath = path.join(rdraDir, fileName);
    if (!fs.existsSync(filePath)) {
        console.log(`Warning: ${fileName} not found, skipping...`);
        return '';
    }

    const lines = readTsvLines(filePath);
    if (lines.length === 0) return '';

    const inHeader = splitTsv(lines[0]);
    const dataLines = lines.slice(1);

    let out = '';
    // Aカラムの分類を元に戻す: A=シート名, C=ヘッダ, B=データ
    out += `A\t${title}\n`;
    out += `C\t${outHeader.join('\t')}\n`;

    for (const line of dataLines) {
        const cols = splitTsv(line);
        const outCols = mapRow(cols, inHeader);

        // 列数を必ず outHeader に合わせる（足りない分は空文字で埋める）
        while (outCols.length < outHeader.length) outCols.push('');
        if (outCols.length > outHeader.length) outCols.length = outHeader.length;

        out += `B\t${outCols.join('\t')}\n`;
    }

    // シート名の上の行をあけない（セクション間の空行を作らない）
    return out;
}

function integrateAsRDRASheet() {
    const systemName = readSystemName();

    const sections = [
        {
            title: 'アクター',
            fileName: 'アクター.tsv',
            outHeader: ['アクター群', 'アクター', '説明'],
            mapRow: (cols) => {
                const actorGroup = cols[0] ?? '';
                const actor = cols[1] ?? '';
                const role = cols[2] ?? '';
                const inOut = cols[3] ?? '';
                const stance = cols[4] ?? '';
                const mainDuty = cols[5] ?? '';

                const desc = joinNonEmpty([
                    role ? `役割:${role}` : '',
                    inOut ? `社内外:${inOut}` : '',
                    stance ? `立場:${stance}` : '',
                    mainDuty ? `主担当業務:${mainDuty}` : ''
                ]);

                return [actorGroup, actor, desc];
            }
        },
        {
            title: '外部システム',
            fileName: '外部システム.tsv',
            outHeader: ['外部システム群', '外部システム', '説明'],
            mapRow: (cols) => {
                const group = cols[0] ?? '';
                const name = cols[1] ?? '';
                const role = cols[2] ?? '';
                return [group, name, role];
            }
        },
        {
            title: '情報',
            fileName: '情報.tsv',
            outHeader: ['コンテキスト', '情報', '属性', '関連情報', '状態モデル', 'バリエーション', '説明'],
            mapRow: (cols) => {
                const context = cols[0] ?? '';
                const info = cols[1] ?? '';
                const attrs = cols[2] ?? '';
                const relInfo = cols[3] ?? '';
                const stateModel = cols[4] ?? '';
                const variation = cols[5] ?? '';
                const desc = cols[6] ?? '';
                return [context, info, attrs, relInfo, stateModel, variation, desc];
            }
        },
        {
            title: '状態',
            fileName: '状態.tsv',
            outHeader: ['コンテキスト', '状態モデル', '状態', '遷移UC', '遷移先状態', '状態モデル・状態の説明'],
            mapRow: (cols) => {
                const context = cols[0] ?? '';
                const model = cols[1] ?? '';
                const state = cols[2] ?? '';
                const uc = cols[3] ?? '';
                const nextState = cols[4] ?? '';
                const desc = cols[5] ?? '';
                return [context, model, state, uc, nextState, desc];
            }
        },
        {
            title: '条件',
            fileName: '条件.tsv',
            outHeader: ['コンテキスト', '条件', '条件の説明', 'バリエーション', '状態モデル'],
            mapRow: (cols) => [cols[0] ?? '', cols[1] ?? '', cols[2] ?? '', cols[3] ?? '', cols[4] ?? '']
        },
        {
            title: 'バリエーション',
            fileName: 'バリエーション.tsv',
            outHeader: ['コンテキスト', 'バリエーション', '値', '説明'],
            mapRow: (cols) => [cols[0] ?? '', cols[1] ?? '', cols[2] ?? '', cols[3] ?? '']
        },
        {
            title: 'BUC',
            fileName: 'BUC.tsv',
            outHeader: [
                '業務',
                'BUC',
                '先',
                'アクティビティ',
                '次',
                'UC',
                '関連モデル1',
                '関連オブジェクト1',
                '関連モデル2',
                '関連オブジェクト2',
                '説明'
            ],
            mapRow: (cols) => [
                cols[0] ?? '',
                cols[1] ?? '',
                cols[2] ?? '',
                cols[3] ?? '',
                cols[4] ?? '',
                cols[5] ?? '',
                cols[6] ?? '',
                cols[7] ?? '',
                cols[8] ?? '',
                cols[9] ?? '',
                cols[10] ?? ''
            ]
        }
    ];

    let integratedContent = '';

    if (systemName) {
        // システム名はA1カラムにセット（1セルのみ）
        integratedContent += `${systemName}\n`;
    }

    for (const section of sections) {
        integratedContent += emitSection(section);
    }

    fs.writeFileSync(outputFile, integratedContent, 'utf-8');
    console.log(`統合ファイル ${outputFile} を作成しました`);
    return outputFile;
}

// 実行
integrateAsRDRASheet();
