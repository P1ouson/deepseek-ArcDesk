import { app } from "./bridge";

export function useController() {
  return async (msg: string) => {
    await app.Submit(msg);
  };
}
