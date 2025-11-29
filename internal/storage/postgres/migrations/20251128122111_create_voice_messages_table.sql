-- +goose Up
-- +goose StatementBegin
CREATE TABLE voice_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  s3_key VARCHAR(512) NOT NULL,
  duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0 AND duration_seconds <= 15),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_voice_messages_room_id_created_at ON voice_messages(room_id, created_at DESC);
CREATE INDEX idx_voice_messages_sender_id ON voice_messages(sender_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS voice_messages;

DROP INDEX IF EXISTS idx_voice_messages_room_id_created_at;
DROP INDEX IF EXISTS idx_voice_messages_sender_id;
-- +goose StatementEnd
