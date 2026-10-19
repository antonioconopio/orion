import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone", // a self-contained server for the Docker image (see Dockerfile)
};

export default nextConfig;
