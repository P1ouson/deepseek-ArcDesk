import { test, expect } from "@playwright/test";

test.describe("ARCDESK workbench smoke", () => {
  test("shell renders with sidebar and composer", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator(".workbench")).toBeVisible();
    await expect(page.locator(".sidebar__mode-toggle")).toBeVisible();
    await expect(page.locator(".composer-shell, .floating-composer, textarea").first()).toBeVisible();
  });

  test("write mode shows workspace UI", async ({ page }) => {
    await page.goto("/");
    await page.locator(".sidebar__mode-btn").nth(1).click();
    await expect(page.locator(".mode-center--write")).toBeVisible();
    await expect(page.locator(".write-sidebar")).toBeVisible();
  });

  test("schedule mode shows task table", async ({ page }) => {
    await page.goto("/");
    await page.locator(".sidebar__action").filter({ hasText: /Schedule|定时/ }).click();
    await expect(page.locator(".schedule-tasks")).toBeVisible();
  });

  test("file preview opens beside chat when clicking a file", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: /工作|Work/i }).click();
    await page.getByRole("tab", { name: /文件|Files/i }).click();
    await page.locator(".files-panel__node--file").first().click();
    await expect(page.locator(".file-preview-panel")).toBeVisible();
    await expect(page.locator(".workbench__body--preview-open")).toBeVisible();
  });
});
