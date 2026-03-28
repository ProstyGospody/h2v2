import { useState } from "react";

export function useNotice() {
  const [message, setMessage] = useState("");

  function notify(value: string) {
    setMessage(value);
  }

  function clear() {
    setMessage("");
  }

  return { message, notify, clear };
}
