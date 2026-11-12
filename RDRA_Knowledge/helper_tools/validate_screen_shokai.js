const fs = require('fs');
const data = JSON.parse(fs.readFileSync('2_RDRASpec/画面照会.json', 'utf-8'));

console.log('### 画面照会.json 検証結果 ###');
console.log('');
console.log('## チェックリスト');
console.log('');

// 1. 全画面を収集
const allScreensInBusinesses = new Set();
data.businesses.forEach(b => {
  b.BUCs.forEach(buc => {
    buc.actors.forEach(actor => {
      actor.screen_names.forEach(s => allScreensInBusinesses.add(s));
    });
  });
});

const allScreensInActorGroups = new Set();
data.actorGroups.forEach(g => {
  g.actors.forEach(actor => {
    actor.screen_names.forEach(s => allScreensInActorGroups.add(s));
  });
});

const allScreensInScreens = new Set();
data.screens.forEach(s => allScreensInScreens.add(s.screen_name));

console.log('[✓] businesses内の画面数:', allScreensInBusinesses.size);
console.log('[✓] actorGroups内の画面数:', allScreensInActorGroups.size);
console.log('[✓] screens内の画面数:', allScreensInScreens.size);
console.log('');

// 2. 画面-アクター関係を確認
const relationData = fs.readFileSync('1_RDRA/関連データ.txt', 'utf-8');
const lines = relationData.split('\n');
const screenActorPairs = new Set();

lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#edge' && parts[1] === '画面' && parts[2] === 'アクター') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [screen, actor] = pair.split('@@');
      if (screen && actor) {
        screenActorPairs.add(screen + '@@' + actor);
      }
    });
  }
});

console.log('[✓] 関連データ.txtの画面-アクター定義数:', screenActorPairs.size);

// 生成されたデータの画面-アクター関係をカウント
let generatedPairsCount = 0;
const generatedPairs = new Set();
data.businesses.forEach(b => {
  b.BUCs.forEach(buc => {
    buc.actors.forEach(actor => {
      actor.screen_names.forEach(screen => {
        generatedPairs.add(screen + '@@' + actor.actor_name);
        generatedPairsCount++;
      });
    });
  });
});

console.log('[✓] 生成された画面-アクター関係数:', generatedPairsCount);
console.log('[✓] ユニークな画面-アクター関係数:', generatedPairs.size);
console.log('');

// 3. 同一画面に複数アクターがいるケース確認
const screenActorsMap = new Map();
data.businesses.forEach(b => {
  b.BUCs.forEach(buc => {
    buc.actors.forEach(actor => {
      actor.screen_names.forEach(screen => {
        if (!screenActorsMap.has(screen)) screenActorsMap.set(screen, new Set());
        screenActorsMap.get(screen).add(actor.actor_name);
      });
    });
  });
});

let multiActorScreenCount = 0;
screenActorsMap.forEach((actors, screen) => {
  if (actors.size > 1) {
    multiActorScreenCount++;
  }
});

console.log('[✓] 複数アクターが関与する画面数:', multiActorScreenCount);
console.log('');

// 4. data_accessの確認
let screensWithDataAccess = 0;
let screensWithNoDataAccess = 0;
let totalDataAccess = 0;
const screensNoDataAccess = [];

data.screens.forEach(s => {
  if (s.data_access.length > 0) {
    screensWithDataAccess++;
    totalDataAccess += s.data_access.length;
  } else {
    screensWithNoDataAccess++;
    screensNoDataAccess.push(s.screen_name);
  }
});

console.log('[✓] data_accessが設定された画面数:', screensWithDataAccess);
console.log('[✓] data_accessが未設定の画面数:', screensWithNoDataAccess);
if (screensNoDataAccess.length > 0) {
  console.log('    未設定画面:', screensNoDataAccess.join(', '));
}
console.log('[✓] 総data_access定義数:', totalDataAccess);
console.log('');

console.log('## 業務別統計');
data.businesses.forEach(b => {
  console.log('業務:', b.business_name);
  console.log('  BUC数:', b.BUCs.length);
  let totalActors = 0;
  let totalScreens = 0;
  b.BUCs.forEach(buc => {
    totalActors += buc.actors.length;
    buc.actors.forEach(actor => {
      totalScreens += actor.screen_names.length;
    });
  });
  console.log('  総アクター数:', totalActors);
  console.log('  総画面数:', totalScreens);
});
console.log('');

console.log('## アクター群別統計');
data.actorGroups.forEach(g => {
  console.log('アクター群:', g.group_name);
  console.log('  アクター数:', g.actors.length);
  let totalScreens = 0;
  g.actors.forEach(a => {
    totalScreens += a.screen_names.length;
  });
  console.log('  総画面数:', totalScreens);
});
console.log('');

console.log('### 検証完了 ###');
