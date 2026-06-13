import { app } from "./bridge";

export function useSubmit() {
  return async (msg: string) => {
    await app.Submit(msg);
  };
}
