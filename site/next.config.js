import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const siteRoot = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(siteRoot, "..");

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Pin turbopack to the monorepo root so hoisted `next` resolves (avoids
  // intermittent "Next.js package not found" HMR panics in workspaces).
  turbopack: {
    root: workspaceRoot,
    resolveAlias: {
      next: resolve(workspaceRoot, "node_modules/next"),
    },
  },
};

export default nextConfig;
