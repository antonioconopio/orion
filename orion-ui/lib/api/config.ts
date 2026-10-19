// NEXT_PUBLIC_ variables are inlined into the browser bundle at build time.
export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
export const WS_URL = API_URL.replace(/^http/, "ws") + "/ws";
