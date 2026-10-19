"use client";

import { useEffect, useState } from "react";

/** The current time in ms, refreshed every `intervalMs`, so "4 minutes ago" and running durations keep moving. */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}
