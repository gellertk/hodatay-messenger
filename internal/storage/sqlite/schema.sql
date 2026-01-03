-- Поручения
CREATE TABLE IF NOT EXISTS matters (
  id INTEGER PRIMARY KEY,
  title TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Чаты
CREATE TABLE IF NOT EXISTS chats (
  id INTEGER PRIMARY KEY,
  matter_id INTEGER NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (matter_id) REFERENCES matters(id)
);

CREATE INDEX IF NOT EXISTS idx_chats_matter_id ON chats(matter_id);

-- Чаты и Участники
CREATE TABLE IF NOT EXISTS chat_participants (
  chat_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  last_read_message_id INTEGER NOT NULL DEFAULT -1,

  PRIMARY KEY (chat_id, user_id),
  FOREIGN KEY (chat_id) REFERENCES chats(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_chat_participants_user ON chat_participants(user_id);

-- Сообщения
CREATE TABLE IF NOT EXISTS messages (
  id          INTEGER PRIMARY KEY,
  chat_id     INTEGER NOT NULL,
  sender_user_id INTEGER NOT NULL,
  text        TEXT NOT NULL,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (chat_id) REFERENCES chats(id)
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_created ON messages(chat_id, created_at);
