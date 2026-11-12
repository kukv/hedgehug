import "@fontsource/ibm-plex-sans-jp/400.css";
import "@fontsource/ibm-plex-sans-jp/500.css";
import "@fontsource/ibm-plex-sans-jp/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@hedgehug/ui/tokens.css";
import "@hedgehug/ui/base.css";
import { setToken, tokenFromHash } from "@hedgehug/api";
import { mount } from "svelte";
import App from "./App.svelte";

setToken(tokenFromHash(location.hash) ?? "");

const target = document.getElementById("app");
if (!target) throw new Error("#app がありません");
mount(App, { target });
