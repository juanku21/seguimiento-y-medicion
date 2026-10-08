import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Desactiva la generación automática de AGENTS.md y CLAUDE.md en next dev (Principio IX, ámbito f)
  agentRules: false,
};

export default nextConfig;
