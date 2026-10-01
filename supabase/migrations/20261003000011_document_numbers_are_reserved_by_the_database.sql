create table internal.document_number_sequence (
  company_id uuid not null references public.company on delete cascade,
  number_prefix text not null check (btrim(number_prefix) <> ''),
  last_sequence integer not null check (last_sequence > 0),
  primary key (company_id, number_prefix)
);

alter table internal.document_number_sequence enable row level security;

create function public.reserve_document_number(target_company uuid, requested_prefix text)
returns text
language plpgsql
volatile
security definer
set search_path = ''
as $$
declare
  reserved integer;
begin
  if public.data_room_member(target_company) is null then
    raise insufficient_privilege using message = 'only an active colleague of the company reserves a document number';
  end if;
  if btrim(coalesce(requested_prefix, '')) = '' then
    raise invalid_parameter_value using message = 'a document number needs a prefix';
  end if;

  insert into internal.document_number_sequence as sequence (company_id, number_prefix, last_sequence)
  select target_company, requested_prefix, coalesce(max(substr(document.document_number, length(requested_prefix) + 1)::integer), 0) + 1
  from public.company_document document
  where document.company_id = target_company
    and left(document.document_number, length(requested_prefix)) = requested_prefix
    and substr(document.document_number, length(requested_prefix) + 1) ~ '^[0-9]{1,9}$'
  on conflict (company_id, number_prefix) do update
    set last_sequence = greatest(sequence.last_sequence + 1, excluded.last_sequence)
  returning sequence.last_sequence into reserved;

  return requested_prefix || lpad(reserved::text, 3, '0');
end;
$$;

revoke all on function public.reserve_document_number(uuid, text) from public, anon;
grant execute on function public.reserve_document_number(uuid, text) to authenticated, service_role;
