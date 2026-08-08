create table public.push_device (
  member_id uuid not null references public.member on delete cascade,
  kind text not null check (kind in ('web-push', 'apns', 'fcm')),
  address text not null,
  keys jsonb not null default '{}'::jsonb,
  primary key (kind, address)
);

create index push_device_member on public.push_device (member_id);

alter table public.push_device enable row level security;

create policy push_device_kept_by_owner on public.push_device
  for all to authenticated
  using (member_id = public.my_member())
  with check (member_id = public.my_member());

alter table public.member add column notification_settings jsonb not null default '{}'::jsonb;
