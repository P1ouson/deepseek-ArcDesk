import { useSubmit } from "./lib/useSubmit";

export default function AppPanel() {
  const submit = useSubmit();
  return (
    <button onClick={() => submit("hi")}>Send</button>
  );
}
