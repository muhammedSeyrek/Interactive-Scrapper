INSERT INTO users (username, password_hash, role)
VALUES ('admin', '$2a$14$1ov4vtubf0hMWJAKy1ZL1OlHlqhnLhDkk6feUm.WOb6S1/UpFrAXC', 'admin')
ON CONFLICT (username) DO NOTHING;