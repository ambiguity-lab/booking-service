INSERT INTO users (id, email, password_hash) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'alice@example.com',   '$2a$10$dummyhashalice0000000000000000000000000000000000'),
    ('b1f9c0da-1d2e-4f3a-8c4b-5e6f7a8b9c0d', 'bob@example.com',     '$2a$10$dummyhashbob000000000000000000000000000000000000');

INSERT INTO properties (id, name, city, country, description, base_price_cents, rating, total_rooms) VALUES
    ('c2e3f4a5-b6c7-4d8e-9f0a-1b2c3d4e5f60', 'Seaside Resort',     'Barcelona',  'Spain',        'A beautiful seaside resort with stunning views of the Mediterranean.', 12000, 4.5, 50),
    ('d3f4a5b6-c7d8-4e9f-0a1b-2c3d4e5f6a71', 'Mountain Lodge',     'Zermatt',    'Switzerland',  'Cozy alpine lodge nestled at the foot of the Matterhorn.', 18500, 4.8, 20),
    ('e4a5b6c7-d8e9-4f0a-1b2c-3d4e5f6a7b82', 'City Center Hotel',  'Tokyo',      'Japan',        'Modern hotel in the heart of Shibuya, steps from the crossing.', 9500,  4.2, 120);
