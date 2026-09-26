create table public.empty_box (
  public_key text primary key check (public_key ~ '^[A-Za-z0-9_-]{43}$'),
  encryption_key text not null check (encryption_key ~ '^[A-Za-z0-9_-]{43}$'),
  public_address text not null,
  announced_at timestamptz not null default now()
);

create index empty_box_at_address on public.empty_box (public_address, announced_at desc);

alter table public.empty_box enable row level security;

revoke all on public.empty_box from public, anon, authenticated;
grant select, insert, update, delete on public.empty_box to service_role;

comment on table public.empty_box is
  'a box that announced its keys and belongs to no company yet; claiming it moves its key onto the company''s fleet credential and deletes the row';
