import { mkdir, readFile, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { dirname, join } from "node:path";

export type CliksConfig = {
  apiUrl: string;
  wsUrl: string;
  currentTeamCode?: string;
  nickname?: string;
  teams: Array<{ code: string; name?: string; lastJoinedAt: string }>;
  sharing: {
    keyboard: boolean;
    mouse: boolean;
  };
  listening: {
    keyboard: boolean;
    mouse: boolean;
    self: boolean;
    volume: number;
    muted: boolean;
    spatial: boolean;
    fatigueProtection: boolean;
    density: number;
  };
  batchWindowMs: number;
};

export const productionApiUrl = "https://139.59.29.207.sslip.io";

const defaultApiUrl = process.env.CLIKS_API_URL ?? productionApiUrl;

export function configPath() {
  return join(process.env.XDG_CONFIG_HOME ?? join(homedir(), ".config"), "cliks", "config.json");
}

export function defaultConfig(): CliksConfig {
  return {
    apiUrl: defaultApiUrl,
    wsUrl: process.env.CLIKS_WS_URL ?? toWsUrl(defaultApiUrl),
    teams: [],
    sharing: {
      keyboard: true,
      mouse: true
    },
    listening: {
      keyboard: true,
      mouse: true,
      self: false,
      volume: 0.7,
      muted: false,
      spatial: true,
      fatigueProtection: true,
      density: 0.8
    },
    batchWindowMs: 500
  };
}

export function toWsUrl(apiUrl: string) {
  return apiUrl.replace(/^http/, "ws").replace(/\/$/, "") + "/ws";
}

export async function loadConfig(): Promise<CliksConfig> {
  try {
    const parsed = JSON.parse(await readFile(configPath(), "utf8")) as Partial<CliksConfig>;
    return {
      ...defaultConfig(),
      ...parsed,
      sharing: { ...defaultConfig().sharing, ...parsed.sharing },
      listening: { ...defaultConfig().listening, ...parsed.listening },
      teams: parsed.teams ?? []
    };
  } catch {
    return defaultConfig();
  }
}

export async function saveConfig(config: CliksConfig) {
  const path = configPath();
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, JSON.stringify(config, null, 2) + "\n", "utf8");
}

export async function rememberTeam(input: { code: string; name?: string }) {
  const config = await loadConfig();
  const code = input.code.toUpperCase();
  const teams = config.teams.filter((team) => team.code !== code);
  teams.unshift({ code, name: input.name, lastJoinedAt: new Date().toISOString() });
  config.teams = teams.slice(0, 12);
  config.currentTeamCode = code;
  await saveConfig(config);
  return config;
}
