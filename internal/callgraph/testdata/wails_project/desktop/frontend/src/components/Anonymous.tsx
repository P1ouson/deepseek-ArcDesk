import { app } from "../lib/bridge";

export default function Anonymous() {
  return <button onClick={() => { app.Submit("x"); }}>Go</button>;
}
