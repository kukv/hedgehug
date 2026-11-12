const fs = require('fs');

// ファイル読込
const relationData = fs.readFileSync('1_RDRA/関連データ.txt', 'utf-8');
const dataModel = fs.readFileSync('2_RDRASpec/論理データモデル.md', 'utf-8');
const screenList = JSON.parse(fs.readFileSync('2_RDRASpec/画面一覧.json', 'utf-8'));

// 関連データから関係を抽出
const lines = relationData.split('\n');

// 業務→BUC抽出
const businessBucMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#child' && parts[1] === '業務' && parts[2] === 'BUC') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [business, buc] = pair.split('@@');
      if (business && buc) {
        if (!businessBucMap.has(business)) businessBucMap.set(business, new Set());
        businessBucMap.get(business).add(buc);
      }
    });
  }
});

// BUC→アクティビティ
const bucActivityMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#child' && parts[1] === 'BUC' && parts[2] === 'アクティビティ') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [buc, activity] = pair.split('@@');
      if (buc && activity) {
        if (!bucActivityMap.has(buc)) bucActivityMap.set(buc, new Set());
        bucActivityMap.get(buc).add(activity);
      }
    });
  }
});

// アクティビティ→UC
const activityUcMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#edge' && parts[1] === 'アクティビティ' && parts[2] === 'UC') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [activity, uc] = pair.split('@@');
      if (activity && uc) {
        if (!activityUcMap.has(activity)) activityUcMap.set(activity, new Set());
        activityUcMap.get(activity).add(uc);
      }
    });
  }
});

// UC→画面
const ucScreenMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#edge' && parts[1] === 'UC' && parts[2] === '画面') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [uc, screen] = pair.split('@@');
      if (uc && screen) {
        if (!ucScreenMap.has(uc)) ucScreenMap.set(uc, new Set());
        ucScreenMap.get(uc).add(screen);
      }
    });
  }
});

// 画面→アクター
const screenActorMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#edge' && parts[1] === '画面' && parts[2] === 'アクター') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [screen, actor] = pair.split('@@');
      if (screen && actor) {
        if (!screenActorMap.has(screen)) screenActorMap.set(screen, new Set());
        screenActorMap.get(screen).add(actor);
      }
    });
  }
});

// アクター群→アクター
const actorGroupMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#child' && parts[1] === 'アクター群' && parts[2] === 'アクター') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [group, actor] = pair.split('@@');
      if (group && actor) {
        if (!actorGroupMap.has(group)) actorGroupMap.set(group, new Set());
        actorGroupMap.get(group).add(actor);
      }
    });
  }
});

// UC→情報（データアクセス用）
const ucInfoMap = new Map();
lines.forEach(line => {
  const parts = line.split('\t');
  if (parts[0] === '#edge' && parts[1] === 'UC' && parts[2] === '情報') {
    const pairs = parts[3]?.split('//') || [];
    pairs.forEach(pair => {
      const [uc, info] = pair.split('@@');
      if (uc && info) {
        if (!ucInfoMap.has(uc)) ucInfoMap.set(uc, new Set());
        ucInfoMap.get(uc).add(info);
      }
    });
  }
});

// UCの説明から操作種類（CRUD）を推測
function guessCRUD(ucName, entityName) {
  const crud = [];
  const lowerUc = ucName;

  if (lowerUc.includes('登録') || lowerUc.includes('作成') || lowerUc.includes('発行')) {
    crud.push('C');
  }
  if (lowerUc.includes('参照') || lowerUc.includes('確認') || lowerUc.includes('照会')) {
    crud.push('R');
  }
  if (lowerUc.includes('更新') || lowerUc.includes('変更') || lowerUc.includes('調整') || lowerUc.includes('計算')) {
    crud.push('U');
  }
  if (lowerUc.includes('削除')) {
    crud.push('D');
  }

  // デフォルトは参照
  if (crud.length === 0) {
    crud.push('R');
  }

  return crud;
}

// Entityのマッピング（情報名→Entity名）
const infoToEntityMap = {
  '介護会員': 'KaigokaiinEntity',
  '個人情報': 'KojinjohoEntity',
  'サービススタッフ': 'ServiceStaffEntity',
  'スキル': 'SkillEntity',
  '働き方分類': 'ServiceStaffEntity',
  '介護施設': 'KaigoShisetsuEntity',
  '事業所': 'JigyoshoEntity',
  '訪問介護計画': 'HomonKaigoKeikakuEntity',
  '勤務スケジュール': 'KinmuScheduleEntity',
  '訪問介護記録': 'HomonKaigoKirokuEntity',
  '介護費用': 'KaigoHiyoEntity',
  '請求書': 'SeikyushoEntity',
  '回収記録': 'KaishuKirokuEntity',
  '法規情報': 'HokiJohoEntity'
};

// 画面照会.json生成開始
const output = {
  businesses: [],
  actorGroups: [],
  screens: []
};

// 1. businesses → BUCs → actors → screen_names
businessBucMap.forEach((bucs, businessName) => {
  const businessObj = {
    business_name: businessName,
    BUCs: []
  };

  bucs.forEach(bucName => {
    const bucObj = {
      buc_name: bucName,
      actors: []
    };

    // BUCから画面とアクターを辿る
    const actorScreensMap = new Map();

    const activities = bucActivityMap.get(bucName) || new Set();
    activities.forEach(activity => {
      const ucs = activityUcMap.get(activity) || new Set();
      ucs.forEach(uc => {
        const screens = ucScreenMap.get(uc) || new Set();
        screens.forEach(screen => {
          const actors = screenActorMap.get(screen) || new Set();
          actors.forEach(actor => {
            if (!actorScreensMap.has(actor)) actorScreensMap.set(actor, new Set());
            actorScreensMap.get(actor).add(screen);
          });
        });
      });
    });

    // アクターごとに画面をまとめる
    actorScreensMap.forEach((screens, actorName) => {
      bucObj.actors.push({
        actor_name: actorName,
        screen_names: Array.from(screens)
      });
    });

    if (bucObj.actors.length > 0) {
      businessObj.BUCs.push(bucObj);
    }
  });

  if (businessObj.BUCs.length > 0) {
    output.businesses.push(businessObj);
  }
});

// 2. actorGroups → actors → screen_names
actorGroupMap.forEach((actors, groupName) => {
  const groupObj = {
    group_name: groupName,
    actors: []
  };

  actors.forEach(actorName => {
    const actorScreens = new Set();

    // このアクターが使う全画面を収集
    screenActorMap.forEach((actorSet, screenName) => {
      if (actorSet.has(actorName)) {
        actorScreens.add(screenName);
      }
    });

    if (actorScreens.size > 0) {
      groupObj.actors.push({
        actor_name: actorName,
        screen_names: Array.from(actorScreens)
      });
    }
  });

  if (groupObj.actors.length > 0) {
    output.actorGroups.push(groupObj);
  }
});

// 3. screens: 画面一覧に data_access を追加
screenList.screens.forEach(screen => {
  const screenName = screen.screen_name;
  const screenObj = {
    screen_name: screenName,
    fields: screen.fields,
    screen_datas: screen.screen_datas,
    operation: screen.operation,
    data_access: []
  };

  // この画面を使うUCを探す
  const relatedUCs = new Set();
  ucScreenMap.forEach((screens, uc) => {
    if (screens.has(screenName)) {
      relatedUCs.add(uc);
    }
  });

  // UCから情報を取得
  const entityAccessMap = new Map();
  relatedUCs.forEach(uc => {
    const infos = ucInfoMap.get(uc) || new Set();
    infos.forEach(info => {
      const entityName = infoToEntityMap[info];
      if (entityName) {
        if (!entityAccessMap.has(entityName)) {
          entityAccessMap.set(entityName, {
            entity: entityName,
            CRUD: new Set(),
            isMain: false
          });
        }
        const crud = guessCRUD(uc, entityName);
        crud.forEach(c => entityAccessMap.get(entityName).CRUD.add(c));
      }
    });
  });

  // isMain判定：最初のエンティティをメインに
  let isFirst = true;
  entityAccessMap.forEach((access, entity) => {
    screenObj.data_access.push({
      entity: access.entity,
      CRUD: Array.from(access.CRUD),
      isMain: isFirst
    });
    isFirst = false;
  });

  output.screens.push(screenObj);
});

// 出力
const outputJson = JSON.stringify(output, null, 2);
fs.writeFileSync('2_RDRASpec/画面照会.json', outputJson, 'utf-8');

console.log('画面照会.json を生成しました');
console.log('業務数:', output.businesses.length);
console.log('アクター群数:', output.actorGroups.length);
console.log('画面数:', output.screens.length);
