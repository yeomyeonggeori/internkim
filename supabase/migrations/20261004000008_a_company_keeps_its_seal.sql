-- The seal a company stamps its forms with is the company's own, kept once beside
-- its picture: the path of an image in the company folder of its shared scope,
-- which only an administrator may write (20260822000001). Every form that
-- prints the seal reads it from here, so nobody carries a copy of it around.
--
-- The column inherits the table's grants and row level security, the way
-- profile is (20260903000005). A replaced or removed seal leaves its path in
-- abandoned_asset for the sweeper, as a replaced picture does (20260822000000).
alter table public.company add column seal_image text;

alter table public.company
  add constraint company_seal_image_is_in_its_company_folder
  check (
    seal_image is null or (
      public.asset_company(seal_image) is not distinct from id
      and public.asset_scope(seal_image) is not distinct from 'shared'
      and public.asset_segment(seal_image, 3) is not distinct from 'company'
    )
  );

create function public.abandon_the_seal_image()
returns trigger
language plpgsql
security definer
set search_path = public
as $$
begin
  if old.seal_image is not null and old.seal_image is distinct from new.seal_image then
    insert into public.abandoned_asset (path) values (old.seal_image)
    on conflict (path) do nothing;
  end if;
  return coalesce(new, old);
end;
$$;

create trigger company_abandons_its_seal_image
  after update of seal_image or delete on public.company
  for each row execute function public.abandon_the_seal_image();

comment on column public.company.seal_image is
  'the seal the company stamps its forms with: a path in the company folder of its shared scope';
