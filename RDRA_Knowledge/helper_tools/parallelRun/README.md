# AI Code Runner

Node.js で Claude / Gemini / Codex CLI を実行するツール群です。  
並行実行とリアルタイム出力（プレフィックス付き）をサポートしています。

## 特徴

- **マルチプロバイダー対応**: Claude / Gemini / Codex を切り替え可能
- **並行実行**: 複数プロンプトを同時実行
- **リアルタイム出力**: プレフィックス付きで各プロセスの出力を識別
- **互換引数対応**: `{input,output}` 形式も受け付けます（※output 側は無視されます）

---

## 並行実行ランナー (parallel-runner.js)

複数のプロンプトファイルを並行実行し、リアルタイムでコンソール出力します。

### 基本的な使い方

```bash
# 入力ファイルを複数指定
node parallel-runner.js prompt/run1.md prompt/run2.md prompt/run3.md

# 互換形式（入力と出力のペアを指定できるが、output 側は無視される）
node parallel-runner.js "{prompt/run1.md,output/run1.tsv}" "{prompt/run2.md,output/run2.tsv}"

# 混在形式（output 側は無視）
node parallel-runner.js prompt/run1.md "{prompt/run2.md,output/custom.tsv}"
```

### オプション

| オプション | 短縮形 | 説明 |
|-----------|-------|------|
| `--provider` | `-P` | AIプロバイダー (claude / gemini / codex) |
| `--model` | `-m` | モデル指定 |
| `--timeout` | | タイムアウト（ミリ秒、デフォルト: 120000） |
| `--help` | `-h` | ヘルプ表示 |

### 出力例

```
files: 3, provider: (default), model: (default), timeout: 120000ms

🚀 並行実行開始
対象ファイル: prompt/run1.md, prompt/run2.md, prompt/run3.md
[run1] ---
[run1] Claude の応答:
[run2] ---
[run2] Claude の応答:
[run3] ---
[run3] Claude の応答:
[run3] 「初期要望.md」を理解しました...
[run3] ```tsv
[run3] アクター	役割
[run3] エンドユーザー	ウォレットアプリを利用して...
[run2] 了解しました。情報を洗い出します...
[run3] ✅ OK (15.2s)
[run2] ✅ OK (17.0s)
[run1] ✅ OK (18.6s)

🏁 並行実行完了 (合計: 18.6秒)

--- summary ---
OK: 3/3
total: 18.6s
```

---

## ファイル構成

```
parallelRun/
├── node-file-runner-V3.js   # ライブラリ（runAIWithPrefix を提供）
├── parallel-runner.js       # 並行実行ランナー
└── README.md
```

---

## API（プログラムから利用）

```javascript
const { 
    runAIWithPrefix, // プレフィックス付きリアルタイム出力
} = require('./node-file-runner-V3');

// プレフィックス付き（並行実行時に便利）
const result2 = await runAIWithPrefix('分析してください', {
    provider: 'claude',
    prefix: 'task1'  // 出力に [task1] プレフィックスが付く
});
```

---

## 注意事項

### Gemini の非対話実行

- Gemini は CLI の仕様差で stdin 方式（pipe/echo）だと対話待ちになり得るため、内部的に `--yolo` を使う非対話方式へ寄せています。
- 無効化したい場合は `options.yolo = false` をコード側から渡してください。

### Windows での実行

- PowerShell で `{input,output}` 形式を使う場合は、引数を `""` で囲んでください：
  ```bash
  node parallel-runner.js "{prompt/run1.md,output/run1.tsv}"
  ```
