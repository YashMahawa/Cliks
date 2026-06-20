import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import bcrypt from "bcryptjs";
import Database from "better-sqlite3";
import { customAlphabet } from "nanoid";
import { randomUUID } from "node:crypto";
import { mkdirSync } from "node:fs";
import { dirname } from "node:path";

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

const makeSuffix = customAlphabet("ABCDEFGHJKLMNPQRSTUVWXYZ23456789", 4);

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
  const sqlitePath = process.env.CLIKS_SQLITE_PATH;

  if (supabaseUrl && serviceRoleKey) {
    return new SupabaseTeamStore(createClient(supabaseUrl, serviceRoleKey));
  }

  if (sqlitePath) {
    return new SqliteTeamStore(sqlitePath);
  }

  return new MemoryTeamStore();
}

class SqliteTeamStore implements TeamStore {
  private db: Database.Database;

  constructor(path: string) {
    mkdirSync(dirname(path), { recursive: true });
    this.db = new Database(path);
    this.db.pragma("journal_mode = WAL");
    this.db.pragma("foreign_keys = ON");
    this.db
      .prepare(
        `create table if not exists cliks_teams (
          id text primary key,
          code text not null unique,
          name text not null,
          delete_password_hash text not null,
          created_at text not null,
          deleted_at text
        )`
      )
      .run();
    this.db
      .prepare(
        `create index if not exists cliks_teams_code_active_idx
          on cliks_teams (code)
          where deleted_at is null`
      )
      .run();
  }

  async createTeam(input: { name: string; deletePassword: string }): Promise<Team> {
    const deletePasswordHash = await bcrypt.hash(input.deletePassword, 12);

    for (let attempt = 0; attempt < 8; attempt += 1) {
      const team = {
        id: randomUUID(),
        code: makeCode(),
        name: input.name,
        created_at: new Date().toISOString(),
        delete_password_hash: deletePasswordHash
      };

      try {
        this.db
          .prepare(
            `insert into cliks_teams (id, code, name, delete_password_hash, created_at)
              values (@id, @code, @name, @delete_password_hash, @created_at)`
          )
          .run(team);
        return toPublicTeam(team);
      } catch (error) {
        if (!isSqliteUniqueError(error)) throw error;
      }
    }

    throw new Error("Could not generate a unique team code");
  }

  async getTeamByCode(code: string): Promise<Team | null> {
    const row = this.db
      .prepare(
        `select id, code, name, created_at
          from cliks_teams
          where code = ? and deleted_at is null
          limit 1`
      )
      .get(code.toUpperCase()) as
      | { id: string; code: string; name: string; created_at: string }
      | undefined;

    return row ? toPublicTeam(row) : null;
  }

  async deleteTeam(input: { code: string; deletePassword: string }): Promise<boolean> {
    const row = this.db
      .prepare(
        `select id, delete_password_hash
          from cliks_teams
          where code = ? and deleted_at is null
          limit 1`
      )
      .get(input.code.toUpperCase()) as
      | { id: string; delete_password_hash: string }
      | undefined;

    if (!row) return false;
    const ok = await bcrypt.compare(input.deletePassword, row.delete_password_hash);
    if (!ok) return false;

    this.db
      .prepare("update cliks_teams set deleted_at = ? where id = ?")
      .run(new Date().toISOString(), row.id);
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
    if (!team || team.deletedAt) return false;
    const ok = await bcrypt.compare(input.deletePassword, team.deletePasswordHash);
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
    if (!data) return false;

    const ok = await bcrypt.compare(input.deletePassword, data.delete_password_hash);
    if (!ok) return false;

    const { error: updateError } = await this.supabase
      .from("cliks_teams")
      .update({ deleted_at: new Date().toISOString() })
      .eq("id", data.id);

    if (updateError) throw new Error(updateError.message);
    return true;
  }
}

function isSqliteUniqueError(error: unknown) {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    (error as { code?: string }).code === "SQLITE_CONSTRAINT_UNIQUE"
  );
}
