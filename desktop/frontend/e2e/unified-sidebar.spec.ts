import { test, expect } from "@playwright/test";

async function openRightSidebar(page: import("@playwright/test").Page) {
  await page.locator(".studio-rail").getByRole("button", { name: /Code|代码/i }).click();
  const toggle = page.locator(".studio-header__right-panel-btn").first();
  await expect(toggle).toBeVisible({ timeout: 15_000 });
  const pressed = await toggle.getAttribute("aria-pressed");
  if (pressed !== "true") {
    await toggle.click();
  }
  await expect(page.locator(".unified-sidebar")).toBeVisible();
}

async function openAddMenuItem(page: import("@playwright/test").Page, name: RegExp) {
  const sidebar = page.locator(".unified-sidebar");
  await sidebar.locator(".unified-sidebar__tab--add").click();
  await page.getByRole("menuitem", { name }).click();
}

async function expectTerminalPaneVisible(sidebar: import("@playwright/test").Locator) {
  await expect(sidebar.locator(".preview-terminal-pane-host:not([hidden]) .preview-terminal-pane")).toBeVisible({
    timeout: 15_000,
  });
}

async function expectBrowserPaneVisible(sidebar: import("@playwright/test").Locator) {
  await expect(sidebar.locator(".preview-browser-pane-host:not([hidden]) .browser-panel")).toBeVisible({
    timeout: 15_000,
  });
}

test.describe("Unified right sidebar controls", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(page.locator(".workbench")).toBeVisible();
  });

  test("top tabs switch primary views", async ({ page }) => {
    await openRightSidebar(page);
    const sidebar = page.locator(".unified-sidebar");
    await expect(sidebar).toBeVisible();

    await openAddMenuItem(page, /New terminal|新建终端/i);
    await expectTerminalPaneVisible(sidebar);

    await sidebar.getByRole("tab", { name: /Changes|改动/i }).click();
    await expect(sidebar.locator(".sidebar-changes-view, .dock-empty-state")).toBeVisible();

    await sidebar.getByRole("tab", { name: /Files|文件/i }).click();
    await expect(sidebar.locator(".files-panel")).toBeVisible();
  });

  test("add menu opens git and todo views", async ({ page }) => {
    await openRightSidebar(page);
    const sidebar = page.locator(".unified-sidebar");
    await openAddMenuItem(page, /^Git$|Git/i);
    await expect(sidebar.locator(".git-panel")).toBeVisible();

    await openAddMenuItem(page, /Overview|概览/i);
    await expect(sidebar.locator(".context-panel")).toBeVisible();

    await openAddMenuItem(page, /To-dos|待办/i);
    await expect(sidebar.locator(".todo-panel, .dock-empty-state")).toBeVisible();
  });

  test("browser session does not spawn duplicate tabs on revisit", async ({ page }) => {
    await openRightSidebar(page);
    const sidebar = page.locator(".unified-sidebar");
    await openAddMenuItem(page, /New browser|新建浏览器/i);
    await expectBrowserPaneVisible(sidebar);
    const sessionTabsAfterFirst = await sidebar.locator(".unified-sidebar__session-tab").count();

    await sidebar.getByRole("tab", { name: /Changes|改动/i }).click();
    await sidebar.locator(".unified-sidebar__session-tab").first().click();
    await expectBrowserPaneVisible(sidebar);
    await expect(sidebar.locator(".unified-sidebar__session-tab")).toHaveCount(sessionTabsAfterFirst);
  });

  test("session tabs list browser and terminal together", async ({ page }) => {
    await openRightSidebar(page);
    const sidebar = page.locator(".unified-sidebar");

    await openAddMenuItem(page, /New browser|新建浏览器/i);
    await expectBrowserPaneVisible(sidebar);

    await openAddMenuItem(page, /New terminal|新建终端/i);
    await expectTerminalPaneVisible(sidebar);
    await expect(sidebar.locator(".unified-sidebar__session-tab")).toHaveCount(2);

    await sidebar.getByRole("tab", { name: /Changes|改动/i }).click();
    await expect(sidebar.locator(".unified-sidebar__session-tab")).toHaveCount(2);

    await sidebar.locator(".unified-sidebar__session-tab").first().click();
    await expectBrowserPaneVisible(sidebar);

    await sidebar.locator(".unified-sidebar__session-tab").nth(1).click();
    await expectTerminalPaneVisible(sidebar);
  });

  test("session tab close buttons stay visible", async ({ page }) => {
    await openRightSidebar(page);
    const sidebar = page.locator(".unified-sidebar");
    await openAddMenuItem(page, /New browser|新建浏览器/i);
    await expect(sidebar.locator(".unified-sidebar__session-tab")).toHaveCount(1);

    const closeBtn = sidebar.locator(".unified-sidebar__session-tab-close").first();
    await expect(closeBtn).toBeVisible();
    await expect(closeBtn).not.toHaveCSS("display", "none");
  });
});
