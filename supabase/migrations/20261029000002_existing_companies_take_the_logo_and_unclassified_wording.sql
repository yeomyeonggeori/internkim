update public.data_room_category category
set description = template.value ->> 'description'
from jsonb_array_elements(internal.data_room_template() -> 'categories') template
where category.code = template.value ->> 'code'
  and (category.code, category.description) in (
    ('GP', 'Company introductions, history, mission and general company plans. Financing presentations belong to RP.'),
    ('SM', 'Brand assets, campaigns, advertising and market research, including images and video.'));

update public.data_room_category
set name = 'Unclassified'
where code = 'X' and name = 'Inbox';

update public.data_room_category
set name_ko = '미분류'
where code = 'X' and name_ko = '분류 대기';
