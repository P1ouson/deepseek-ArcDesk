import { test, expect } from "@playwright/test";

test.describe("ARCDESK workbench smoke", () => {
  test("shell renders with sidebar and composer", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator(".workbench")).toBeVisible();
    await expect(page.locator(".studio-rail")).toBeVisible();
    await expect(page.locator(".composer-shell, .floating-composer, textarea").first()).toBeVisible();
  });

  test("write mode shows workspace UI", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: /写作|Write/i }).click();
    await expect(page.locator(".mode-center--write")).toBeVisible();
    await expect(page.locator(".write-sidebar")).toBeVisible();
  });

  test("schedule mode shows task table", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: /Schedule|定时/i }).click();
    await expect(page.locator(".schedule-tasks")).toBeVisible();
  });

  test("settings general shows close behavior and network", async ({ page }) => {
    await page.goto("/");
    await page.locator(".studio-rail").getByRole("button").filter({ hasText: /设置|Settings/i }).click();
    await expect(page.locator(".settings-studio__main")).toBeVisible();
    await expect(page.getByText(/关闭窗口时|When closing window/i)).toBeVisible();
    await expect(page.getByText(/网络与代理|Network & proxy/i)).toBeVisible();
  });

  test("dev mock banner shows in browser preview", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator(".dev-mock-banner")).toBeVisible();
  });

  test("file preview opens beside chat when clicking a file", async ({ page }) => {
    await page.goto("/");
    await page
      .locator(".topbar__hub")
      .filter({ has: page.locator(".topbar__hub-label", { hasText: /Work|工作/ }) })
      .locator(".topbar__hub-main")
      .click();
    await expect(page.locator(".right-dock")).toBeVisible();
    await page.locator(".right-dock__subtab").filter({ hasText: /文件|Files/i }).click();
    await page.locator(".files-panel__node:not(.files-panel__node--dir)").first().click();
    await expect(page.locator(".file-preview-panel")).toBeVisible();
    await expect(page.locator(".workbench__body--preview-open")).toBeVisible();
  });
});
