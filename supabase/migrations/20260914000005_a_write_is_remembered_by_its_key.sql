-- A retry of a write that already committed must answer what the first call
-- answered rather than write again: on 2026-09-13 a client gave up on task_add
-- 0.4 seconds before the row committed and its retry made the task twice. A
-- write that carries an idempotency key is remembered here, by the member who
-- made it, and the same key from the same member reads the answer back.
-- Only a successful answer is kept, so a refused call can be retried.

create table public.idempotency_key (
  member_id uuid not null references public.member on delete cascade,
  key text not null,
  tool_name text not null,
  response jsonb not null,
  created_at timestamptz not null default now(),
  primary key (member_id, key)
);

alter table public.idempotency_key enable row level security;

create policy idempotency_key_kept_by_owner on public.idempotency_key
  for all to authenticated
  using (member_id = public.my_member())
  with check (member_id = public.my_member());

grant select, insert on public.idempotency_key to authenticated;
revoke update, delete on public.idempotency_key from anon, authenticated;
grant all on public.idempotency_key to service_role;
