#!/usr/bin/env node

const readline = require('readline');
const { createMenuAction } = require('./RDRA_Knowledge/helper_tools/menuAction');

const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
});
rl.setPrompt('> ');

/**
 * メニューを表示する
 */
function showMenu() {
    console.log('■ZeroOne');
    console.log('1.フェーズ単位実行：成果物が無い最初のPhaseを実行（Phase4が揃っていればPhase5を実行）');
    console.log('2.一括要件定義：RDRA定義を一括実行する');
    console.log('■RDRA');
    console.log('11.RDRAGraphを表示：関連データを作成しRDRAGraphを表示');
    console.log('12.Spreadsheetに展開：RDRA定義をクリップボードにコピー');
    console.log('■RDRASpec：仕様');
    console.log('21.仕様の作成：論理データ構造/画面/ビジネスルール');
    console.log('22.BUC・アクター別画面を表示する');
    console.log('■全般');
    console.log('0.メニュー終了');
    console.log('');
    console.log('99.生成した成果物の削除：0_RDRAZeroOne/1_RDRA/2_RDRASpec');
    console.log('');
    console.log('実行したい番号を入力してください');
}

let executeOption;
// 入力の受け口を切り替える（Enter待ち / メニュー入力）
let inputHandler = null;

function handleMenuInput(line) {
    executeOption(line.trim());
}

function waitForEnterThenNext() {
    console.log('続行するにはEnterキーを押してください...');
    inputHandler = () => {
        promptUser();
    };
    rl.prompt();
}

function promptUser() {
    showMenu();
    inputHandler = handleMenuInput;
    rl.prompt();
}

rl.on('line', (line) => {
    if (inputHandler) inputHandler(line);
});

executeOption = createMenuAction({ rl, promptUser, waitForEnterThenNext });

console.log('コマンド実行メニュー');
promptUser();
