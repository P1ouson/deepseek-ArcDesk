import { app } from "../lib/bridge";

export function EdgeCases() {
  const dyn = () => import("./bridge");
  const bracket = () => app["Submit"]();
  const badCase = () => app.submit("x");
  return <button onClick={() => { dyn(); bracket(); badCase(); }}>X</button>;
}
