-- Local development data: two profiles, three items each, one pending offer
-- and one accepted offer, so a full trade can be played through via curl.
-- Login for both accounts: password "swappy-dev".
-- Fixed UUIDs keep curl examples stable.

create extension if not exists pgcrypto;

insert into users (id, email, password_hash) values
    ('00000000-0000-4000-8000-00000000000a', 'anna@swappy.local', crypt('swappy-dev', gen_salt('bf', 10))),
    ('00000000-0000-4000-8000-00000000000b', 'ben@swappy.local',  crypt('swappy-dev', gen_salt('bf', 10)))
on conflict do nothing;

insert into profiles (id, display_name, postal_code, geo_lat, geo_lon, terms_accepted_at) values
    ('00000000-0000-4000-8000-00000000000a', 'anna', '10115', 52.53, 13.38, now()),
    ('00000000-0000-4000-8000-00000000000b', 'ben',  '10245', 52.50, 13.46, now())
on conflict do nothing;

insert into items (id, owner_id, title, description, category, condition, wanted_text, status) values
    ('00000000-0000-4000-8000-0000000000a1', '00000000-0000-4000-8000-00000000000a', 'Rennrad Peugeot 1985', 'Rahmenhöhe 56, fährt gut', 'sport', 'used', 'Gitarre oder Plattenspieler', 'active'),
    ('00000000-0000-4000-8000-0000000000a2', '00000000-0000-4000-8000-00000000000a', 'Kaffeemühle Zassenhaus', 'Handmühle, Mahlwerk wie neu', 'haushalt', 'good', 'Bücher', 'active'),
    ('00000000-0000-4000-8000-0000000000a3', '00000000-0000-4000-8000-00000000000a', 'Brettspiel Carcassonne', 'Vollständig, mit Erweiterung', 'spiele', 'good', null, 'active'),
    ('00000000-0000-4000-8000-0000000000b1', '00000000-0000-4000-8000-00000000000b', 'Westerngitarre Yamaha', 'Neue Saiten', 'musik', 'good', 'Fahrrad', 'active'),
    ('00000000-0000-4000-8000-0000000000b2', '00000000-0000-4000-8000-00000000000b', 'Kochbuch-Sammlung', '12 Bücher, italienisch & asiatisch', 'buecher', 'used', 'Küchengeräte', 'active'),
    ('00000000-0000-4000-8000-0000000000b3', '00000000-0000-4000-8000-00000000000b', 'Zimmerpflanze Monstera', 'ca. 80 cm', 'garten', 'new', null, 'active')
on conflict do nothing;

-- Pending: Ben offers his guitar for Anna's racing bike.
insert into trade_offers (id, initiator_id, responder_id, target_item_id, message, status) values
    ('00000000-0000-4000-8000-0000000000f1', '00000000-0000-4000-8000-00000000000b', '00000000-0000-4000-8000-00000000000a',
     '00000000-0000-4000-8000-0000000000a1', 'Gitarre gegen dein Rad?', 'pending')
on conflict do nothing;
insert into trade_offer_items (offer_id, item_id, side) values
    ('00000000-0000-4000-8000-0000000000f1', '00000000-0000-4000-8000-0000000000b1', 'offered'),
    ('00000000-0000-4000-8000-0000000000f1', '00000000-0000-4000-8000-0000000000a1', 'requested')
on conflict do nothing;

-- Accepted: Anna's coffee grinder for Ben's cookbooks. Both items reserved.
insert into trade_offers (id, initiator_id, responder_id, target_item_id, message, status, accepted_at) values
    ('00000000-0000-4000-8000-0000000000f2', '00000000-0000-4000-8000-00000000000a', '00000000-0000-4000-8000-00000000000b',
     '00000000-0000-4000-8000-0000000000b2', 'Mühle gegen Kochbücher', 'accepted', now())
on conflict do nothing;
insert into trade_offer_items (offer_id, item_id, side) values
    ('00000000-0000-4000-8000-0000000000f2', '00000000-0000-4000-8000-0000000000a2', 'offered'),
    ('00000000-0000-4000-8000-0000000000f2', '00000000-0000-4000-8000-0000000000b2', 'requested')
on conflict do nothing;
update items
   set status = 'reserved', reserved_by_offer_id = '00000000-0000-4000-8000-0000000000f2'
 where id in ('00000000-0000-4000-8000-0000000000a2', '00000000-0000-4000-8000-0000000000b2')
   and status = 'active';
