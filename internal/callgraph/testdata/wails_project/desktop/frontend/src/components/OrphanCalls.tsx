import { app } from "../lib/bridge";

export function OrphanCalls() {
  return <button onClick={() => app.FooNotInDTS("x")}>Orphan</button>;
}
