CREATE TABLE users (
 id BIGINT PRIMARY KEY,
 email TEXT NOT NULL UNIQUE
);
CREATE TABLE orders (
 id BIGINT PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 total numeric(12, 2) NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX users_email_idx ON users (email);
