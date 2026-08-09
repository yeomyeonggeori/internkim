create or replace function public.work_location_names(company_id uuid)
returns text[]
language sql
stable
set search_path = public
as $$
  select array_agg(entry ->> 'name' order by ordinality)
  from public.company
  cross join lateral jsonb_array_elements(company.work_locations) with ordinality as element(entry, ordinality)
  where company.id = work_location_names.company_id;
$$;
