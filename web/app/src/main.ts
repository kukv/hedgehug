import "@fontsource/ibm-plex-sans-jp/400.css";
import "@fontsource/ibm-plex-sans-jp/500.css";
import "@fontsource/ibm-plex-sans-jp/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@hedgehug/ui/tokens.css";
import "@hedgehug/ui/base.css";
import { mount } from "svelte";
import App from "./App.svelte";

// トークンは wire.ts が読む(App.svelte が wire.ts を import するので、ここより先に動く)。
const target = document.getElementById("app");
if (!target) throw new Error("#app がありません");
mount(App, { target });
