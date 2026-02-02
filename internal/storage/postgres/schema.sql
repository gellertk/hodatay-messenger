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
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  reply_to_message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL
);

CREATE INDEX idx_messages_chat_created ON messages(chat_id, created_at);
CREATE INDEX idx_messages_reply_to ON messages(reply_to_message_id) WHERE reply_to_message_id IS NOT NULL;

-- Файлы
CREATE TABLE attachments (
  id BIGSERIAL PRIMARY KEY,
  message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  file_id TEXT NOT NULL,
  content_type TEXT NOT NULL,
  filename TEXT NOT NULL,
  size BIGINT NOT NULL,
  width INT,
  height INT,
  duration_ms BIGINT,
  waveform_u8 TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE attachments
ADD CONSTRAINT attachments_dims_chk
CHECK (
  (width IS NULL AND height IS NULL)
  OR (width > 0 AND height > 0)
);

DROP INDEX IF EXISTS idx_attachments_file_id;
CREATE INDEX idx_attachments_file_id ON attachments(file_id);
CREATE INDEX idx_attachments_message_id_id ON attachments(message_id, id);

-- Uploads
CREATE TABLE uploads (
  id BIGSERIAL PRIMARY KEY,
  file_id TEXT NOT NULL UNIQUE,
  owner_user_id BIGINT NOT NULL,
  original_filename TEXT,
  client_content_type TEXT,
  content_type TEXT,
  size BIGINT,
  width INT,
  height INT,
  duration_ms BIGINT,
  waveform_u8 TEXT,
  status TEXT NOT NULL DEFAULT 'presigned', -- type UploadStatus
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ready_at   TIMESTAMPTZ,
  used_at    TIMESTAMPTZ
);

CREATE INDEX idx_uploads_owner_created ON uploads(owner_user_id, created_at);
CREATE INDEX idx_uploads_status_created ON uploads(status, created_at);
