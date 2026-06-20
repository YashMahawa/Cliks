create table if not exists public.cliks_teams (
  id uuid primary key default gen_random_uuid(),
  code text not null unique,
  name text not null,
  delete_password_hash text not null,
  created_at timestamptz not null default now(),
  deleted_at timestamptz
);

create index if not exists cliks_teams_code_active_idx
  on public.cliks_teams (code)
  where deleted_at is null;

alter table public.cliks_teams enable row level security;

-- The backend should use the Supabase service role key. No public policies are required.
