declare module "../../wailsjs/go/main/App" {
  export {};
}

declare module "../../wailsjs/runtime/runtime" {
  export function WindowSetDarkTheme(): void;
  export function WindowSetLightTheme(): void;
  export function WindowSetSystemDefaultTheme(): void;
  export function WindowSetBackgroundColour(r: number, g: number, b: number, a: number): void;
  export function WindowGetSize(): Promise<{ w: number; h: number }>;
  export function WindowGetPosition(): Promise<{ x: number; y: number }>;
  export function WindowIsMaximised(): Promise<boolean>;
}
