import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "playwright/test";

test("has no automatically detectable accessibility violations", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Operations console" }),
  ).toBeVisible();
  await page.waitForTimeout(600);

  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();

  expect(results.violations).toEqual([]);
});

test("supports blocker review and evidence exploration", async ({ page }) => {
  await page.goto("/");
  await page
    .getByRole("button", {
      name: /AI workflow attempted an unapproved clinical write/,
    })
    .click();

  await expect(
    page.getByRole("heading", {
      name: "AI workflow attempted an unapproved clinical write",
    }),
  ).toBeVisible();

  await page.getByRole("button", { name: /Guardrailed tool trace/ }).click();
  await expect(
    page.getByRole("heading", { name: "Guardrailed tool trace" }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Guardrailed tool trace excerpt"),
  ).toContainText("MedicationRequest.create");
});

test("provides a visible keyboard skip path", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Tab");

  const skipLink = page.getByRole("link", { name: "Skip to main content" });
  await expect(skipLink).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#main-content")).toBeFocused();
});
