-- Поручения
CREATE TABLE matters (
  id BIGSERIAL PRIMARY KEY,
  title TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Чаты
CREATE TABLE chats (
  id BIGSERIAL PRIMARY KEY,
  matter_id BIGINT NOT NULL REFERENCES matters(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chats_matter_id ON chats(matter_id);

-- Чаты и участники
CREATE TABLE chat_participants (
  chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL,
  last_read_message_id BIGINT NOT NULL DEFAULT 0,

  PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX idx_chat_participants_user ON chat_participants(user_id);

-- Сообщения
CREATE TABLE messages (
  id BIGSERIAL PRIMARY KEY,
  chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  sender_user_id BIGINT NOT NULL,
  text TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_chat_created ON messages(chat_id, created_at);

-- Файлы
CREATE TABLE attachments (
  id BIGSERIAL PRIMARY KEY,
  message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  content_type TEXT NOT NULL,
  filename TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_attachments_key ON attachments(key);
CREATE INDEX idx_attachments_message_id ON attachments(message_id);