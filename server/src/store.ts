import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import bcrypt from "bcryptjs";
import { customAlphabet } from "nanoid";
import { randomUUID } from "node:crypto";
import pg from "pg";

export type Team = {
  id: string;
  code: string;
  name: string;
  createdAt: string;
};

export type TeamStore = {
  createTeam(input: { name: string; deletePassword: string }): Promise<Team>;
  getTeamByCode(code: string): Promise<Team | null>;
  deleteTeam(input: { code: string; deletePassword: string }): Promise<boolean>;
};

const makeSuffix = customAlphabet("ABCDEFGHJKLMNPQRSTUVWXYZ23456789", 6);
const dummyDeletePasswordHash = "$2b$12$mMCOaGsqrw5HVe4PboZEdeKqkZZSrer3p4/KmwcbXB0YraVftIwf.";

function makeCode() {
  return `CLIK-${makeSuffix()}`;
}

function toPublicTeam(row: {
  id: string;
  code: string;
  name: string;
  created_at?: string;
  createdAt?: string;
}): Team {
  return {
    id: row.id,
    code: row.code,
    name: row.name,
    createdAt: row.created_at ?? row.createdAt ?? new Date().toISOString()
  };
}

export function createTeamStoreFromEnv(): TeamStore {
  const supabaseUrl = process.env.SUPABASE_URL;
  const serviceRoleKey = process.env.SUPABASE_SERVICE_ROLE_KEY;
  const databaseUrl = process.env.DATABASE_URL;
  const useLocalPostgres = process.env.CLIKS_LOCAL_POSTGRES === "true";

  if (supabaseUrl && serviceRoleKey) {
    return new SupabaseTeamStore(createClient(supabaseUrl, serviceRoleKey));
  }

  if (databaseUrl || useLocalPostgres) {
    return new PostgresTeamStore(databaseUrl);
  }

  return new MemoryTeamStore();
}

class PostgresTeamStore implements TeamStore {
  private pool: pg.Pool;
  private ready: Promise<void>;

  constructor(connectionString?: string) {
    this.pool = new pg.Pool(
      connectionString
        ? { connectionString }
        : {
            host: "/var/run/postgresql",
            database: "cliks",
            user: "cliks"
          }
    );
    this.ready = this.init();
  }

  private async init() {
    await this.pool.query(
      `create table if not exists cliks_teams (
        id uuid primary key,
        code text not null unique,
        name text not null,
        delete_password_hash text not null,
        created_at timestamptz not null default now(),
        deleted_at timestamptz
      )`
    );
    await this.pool.query(
      `create index if not exists cliks_teams_code_active_idx
        on cliks_teams (code)
        where deleted_at is null`
    );
  }

  async createTeam(input: { name: string; deletePassword: string }): Promise<Team> {
    await this.ready;
    const deletePasswordHash = await bcrypt.hash(input.deletePassword, 12);

    for (let attempt = 0; attempt < 8; attempt += 1) {
      const id = randomUUID();
      const code = makeCode();

      try {
        const { rows } = await this.pool.query(
          `insert into cliks_teams (id, code, name, delete_password_hash)
            values ($1, $2, $3, $4)
            returning id, code, name, created_at`,
          [id, code, input.name, deletePasswordHash]
        );
        return toPublicTeam(rows[0]);
      } catch (error) {
        if (!isPostgresUniqueError(error)) throw error;
      }
    }

    throw new Error("Could not generate a unique team code");
  }

  async getTeamByCode(code: string): Promise<Team | null> {
    await this.ready;
    const { rows } = await this.pool.query(
      `select id, code, name, created_at
        from cliks_teams
        where code = $1 and deleted_at is null
        limit 1`,
      [code.toUpperCase()]
    );

    return rows[0] ? toPublicTeam(rows[0]) : null;
  }

  async deleteTeam(input: { code: string; deletePassword: string }): Promise<boolean> {
    await this.ready;
    const { rows } = await this.pool.query(
      `select id, delete_password_hash
        from cliks_teams
        where code = $1 and deleted_at is null
        limit 1`,
      [input.code.toUpperCase()]
    );
    const row = rows[0] as { id: string; delete_password_hash: string } | undefined;

    const ok = await bcrypt.compare(input.deletePassword, row?.delete_password_hash ?? dummyDeletePasswordHash);
    if (!row) return false;
    if (!ok) return false;

    await this.pool.query("update cliks_teams set deleted_at = now() where id = $1", [row.id]);
    return true;
  }
}

class MemoryTeamStore implements TeamStore {
  private teams = new Map<string, Team & { deletePasswordHash: string; deletedAt?: string }>();

  constructor() {
    void this.createTeam({ name: "Local Test Room", deletePassword: "delete-me" }).then((team) => {
      if (team.code !== "CLIK-LOCAL") {
        const saved = this.teams.get(team.code);
        if (saved) {
          this.teams.delete(team.code);
          this.teams.set("CLIK-LOCAL", { ...saved, code: "CLIK-LOCAL" });
        }
      }
    });
  }

  async createTeam(input: { name: string; deletePassword: string }): Promise<Team> {
    const hash = await bcrypt.hash(input.deletePassword, 12);
    let code = makeCode();
    while (this.teams.has(code)) code = makeCode();
    const team = {
      id: randomUUID(),
      code,
      name: input.name,
      createdAt: new Date().toISOString(),
      deletePasswordHash: hash
    };
    this.teams.set(code, team);
    return toPublicTeam(team);
  }

  async getTeamByCode(code: string): Promise<Team | null> {
    const team = this.teams.get(code.toUpperCase());
    if (!team || team.deletedAt) return null;
    return toPublicTeam(team);
  }

  async deleteTeam(input: { code: string; deletePassword: string }): Promise<boolean> {
    const team = this.teams.get(input.code.toUpperCase());
    const ok = await bcrypt.compare(input.deletePassword, team && !team.deletedAt ? team.deletePasswordHash : dummyDeletePasswordHash);
    if (!team || team.deletedAt) return false;
    if (!ok) return false;
    team.deletedAt = new Date().toISOString();
    return true;
  }
}

class SupabaseTeamStore implements TeamStore {
  constructor(private supabase: SupabaseClient) {}

  async createTeam(input: { name: string; deletePassword: string }): Promise<Team> {
    const deletePasswordHash = await bcrypt.hash(input.deletePassword, 12);

    for (let attempt = 0; attempt < 8; attempt += 1) {
      const code = makeCode();
      const { data, error } = await this.supabase
        .from("cliks_teams")
        .insert({
          code,
          name: input.name,
          delete_password_hash: deletePasswordHash
        })
        .select("id, code, name, created_at")
        .single();

      if (!error && data) return toPublicTeam(data);
      if (error?.code !== "23505") {
        throw new Error(error?.message ?? "Could not create team");
      }
    }

    throw new Error("Could not generate a unique team code");
  }

  async getTeamByCode(code: string): Promise<Team | null> {
    const { data, error } = await this.supabase
      .from("cliks_teams")
      .select("id, code, name, created_at")
      .eq("code", code.toUpperCase())
      .is("deleted_at", null)
      .maybeSingle();

    if (error) throw new Error(error.message);
    return data ? toPublicTeam(data) : null;
  }

  async deleteTeam(input: { code: string; deletePassword: string }): Promise<boolean> {
    const { data, error } = await this.supabase
      .from("cliks_teams")
      .select("id, delete_password_hash")
      .eq("code", input.code.toUpperCase())
      .is("deleted_at", null)
      .maybeSingle();

    if (error) throw new Error(error.message);

    const ok = await bcrypt.compare(input.deletePassword, data?.delete_password_hash ?? dummyDeletePasswordHash);
    if (!data) return false;
    if (!ok) return false;

    const { error: updateError } = await this.supabase
      .from("cliks_teams")
      .update({ deleted_at: new Date().toISOString() })
      .eq("id", data.id);

    if (updateError) throw new Error(updateError.message);
    return true;
  }
}

function isPostgresUniqueError(error: unknown) {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    (error as { code?: string }).code === "23505"
  );
}
