import { useCallback, useState } from "react";
import { loadProjectDrawerOpen, saveProjectDrawerOpen } from "./projectDrawerPrefs";

export function useProjectDrawer() {
  const [projectDrawerOpen, setOpen] = useState(loadProjectDrawerOpen);

  const setProjectDrawerOpen = useCallback((next: boolean | ((prev: boolean) => boolean)) => {
    setOpen((prev) => {
      const value = typeof next === "function" ? next(prev) : next;
      saveProjectDrawerOpen(value);
      return value;
    });
  }, []);

  const closeProjectDrawer = useCallback(() => {
    setProjectDrawerOpen(false);
  }, [setProjectDrawerOpen]);

  const toggleProjectDrawer = useCallback(() => {
    setProjectDrawerOpen((open) => !open);
  }, [setProjectDrawerOpen]);

  return { projectDrawerOpen, setProjectDrawerOpen, closeProjectDrawer, toggleProjectDrawer };
}
