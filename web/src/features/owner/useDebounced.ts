import { useEffect, useState } from "react";

// Value that follows `value` after `ms` of quiet (the rate editor's live preview waits 300 ms).
export function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}
