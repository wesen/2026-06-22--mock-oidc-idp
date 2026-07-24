ALTER TABLE durable_invitations ADD COLUMN invitation_id TEXT;

UPDATE durable_invitations
SET invitation_id = json_extract(CAST(data AS TEXT), '$.ID')
WHERE invitation_id IS NULL;

CREATE UNIQUE INDEX durable_invitations_invitation_id_idx
    ON durable_invitations(invitation_id);
