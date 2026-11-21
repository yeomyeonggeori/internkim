create or replace function public.company_keeps_a_backup(kept_company uuid)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1 from storage.objects
    where bucket_id = 'box_backup'
      and (
        name like kept_company::text || '/snapshots/%'
        or name like kept_company::text || '/base/internkim-base-%.json'
      )
  );
$$;

comment on function public.company_keeps_a_backup(uuid) is
  'whether the company''s service box has kept an off-site backup of it: a snapshot, or the base backup its database''s kept WAL starts from. A company that keeps one is never started fresh on another box: it is taken over from a backup or not at all';
