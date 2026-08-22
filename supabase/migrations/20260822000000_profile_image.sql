-- A company, a member and a contact are the three things with a face. Each keeps
-- the path of its picture in the asset bucket rather than a URL: a signed URL
-- expires and a public one changes with the bucket's policy, and neither belongs
-- in a row that outlives both.
alter table public.company add column profile_image text;
alter table public.member add column profile_image text;
alter table public.contact add column profile_image text;

-- The bucket already names who may read what by the path itself, and a face is
-- readable by everyone in the company, which is what the shared scope means. A
-- path the convention does not parse scopes to null, and null = 'shared' is null,
-- which a check constraint counts as satisfied — so the comparison has to be one
-- that answers false.
alter table public.company
  add constraint company_profile_image_is_its_own_shared_asset
  check (
    profile_image is null or (
      public.asset_scope(profile_image) is not distinct from 'shared'
      and public.asset_company(profile_image) is not distinct from id
    )
  );
alter table public.member
  add constraint member_profile_image_is_its_companys_shared_asset
  check (
    profile_image is null or (
      public.asset_scope(profile_image) is not distinct from 'shared'
      and public.asset_company(profile_image) is not distinct from company_id
    )
  );
alter table public.contact
  add constraint contact_profile_image_is_its_companys_shared_asset
  check (
    profile_image is null or (
      public.asset_scope(profile_image) is not distinct from 'shared'
      and public.asset_company(profile_image) is not distinct from company_id
    )
  );

-- A picture whose row is gone is a picture nobody can reach and nobody will
-- delete. Postgres cannot be the one to remove it — storage refuses a delete
-- that does not come through its API:
--
--   ERROR: Direct deletion from storage tables is not allowed.
--
-- So the row that goes leaves the path behind for whoever can, and the record of
-- what is owed outlives the transaction that owed it.
create table public.abandoned_asset (
  path text primary key,
  abandoned_at timestamptz not null default now()
);

-- Grants are not inherited by a table made after them, and row level security
-- with no policy is what says a queue is nobody's to read: the sweeper holds the
-- service key and passes either way, and no member has business here.
grant select, insert, update, delete on public.abandoned_asset to service_role;
alter table public.abandoned_asset enable row level security;

create function public.abandon_the_profile_image()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if old.profile_image is not null and old.profile_image is distinct from new.profile_image then
    insert into public.abandoned_asset (path) values (old.profile_image)
    on conflict (path) do nothing;
  end if;
  return coalesce(new, old);
end;
$$;

create trigger company_abandons_its_profile_image
  after update of profile_image or delete on public.company
  for each row execute function public.abandon_the_profile_image();

create trigger member_abandons_its_profile_image
  after update of profile_image or delete on public.member
  for each row execute function public.abandon_the_profile_image();

create trigger contact_abandons_its_profile_image
  after update of profile_image or delete on public.contact
  for each row execute function public.abandon_the_profile_image();
