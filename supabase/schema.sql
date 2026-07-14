create table if not exists public.cliks_teams (
  id uuid primary key default gen_random_uuid(),
  code text not null,
  name text not null,
  delete_password_hash text not null,
  created_at timestamptz not null default now(),
  last_connected_at timestamptz not null default now(),
  deleted_at timestamptz
);

alter table public.cliks_teams drop constraint if exists cliks_teams_code_key;

alter table public.cliks_teams add column if not exists last_connected_at timestamptz;
update public.cliks_teams set last_connected_at = created_at where last_connected_at is null;
alter table public.cliks_teams alter column last_connected_at set default now();
alter table public.cliks_teams alter column last_connected_at set not null;

create unique index if not exists cliks_teams_code_active_idx
  on public.cliks_teams (code)
  where deleted_at is null;

alter table public.cliks_teams enable row level security;

-- The backend should use the Supabase service role key. No public policies are required.
