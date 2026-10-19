import path from "node:path";
import { defineConfig } from "vitest/config";

process.env.TZ = "UTC"; // the formatters use local time; pin it so expectations are the same everywhere

export default defineConfig({
  test: { environment: "node", include: ["lib/**/*.test.ts"] },
  resolve: { alias: { "@": path.resolve(__dirname) } },
});
