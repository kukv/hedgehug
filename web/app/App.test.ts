import { render, screen } from "@testing-library/svelte";
import { expect, test } from "vitest";
import App from "./App.svelte";

test("App renders the product name", () => {
  render(App);
  expect(screen.getByRole("heading", { name: "Hedgehug" })).toBeInTheDocument();
});
