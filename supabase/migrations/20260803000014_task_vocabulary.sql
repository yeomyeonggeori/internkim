-- What a company calls its work: which business it belongs to, what kind it is,
-- and how big. Read together by every board, so kept together.
alter table public.company add column task_vocabulary jsonb not null default '{}';

alter table public.task
  add column business text,
  add column type text,
  add column size text;

-- A task that has only started has no end yet. Requiring both made the importer
-- invent one, which is worse than an empty cell.
do $$
declare
  paired_nulls text;
begin
  select constraint_name into paired_nulls
  from information_schema.check_constraints
  where constraint_schema = 'public'
    and check_clause like '%starts_at IS NULL%'
    and check_clause like '%ends_at IS NULL%';
  if paired_nulls is not null then
    execute format('alter table public.task drop constraint %I', paired_nulls);
  end if;
end;
$$;
