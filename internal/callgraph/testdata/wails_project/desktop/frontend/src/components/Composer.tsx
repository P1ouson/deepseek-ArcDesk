import { useSubmit } from "./lib/useSubmit";

export default function Composer() {
  const submit = useSubmit();
  return (
    <button onClick={() => submit("hi")}>Send</button>
  );
}
