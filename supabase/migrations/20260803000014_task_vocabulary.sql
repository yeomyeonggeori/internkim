alter table public.company add column task_vocabulary jsonb not null default '{}';

alter table public.task
  add column business text,
  add column type text,
  add column size text;

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
