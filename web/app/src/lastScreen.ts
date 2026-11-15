import { createLastScreen } from "@hedgehug/core";
import { screenStates } from "./search";
import { navigation, screen } from "./wire";

// 最後に開いていた画面(E4a)。出した画面のアドレスを覚え、/ で開いたときはその画面に替える(覚えていなければ
// マイタスク)。wire.ts が 300 行なので、ここで組み立て、App.svelte が読んだときに 1 回だけ始める。/ から移るのは
// 一覧を読む前でもよい(アドレスを替えておけば、screen.start() のあとの解決がその画面を出す)。
createLastScreen({ navigation, current: screen.current, screenStates }).start();
