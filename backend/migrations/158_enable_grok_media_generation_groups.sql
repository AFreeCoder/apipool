-- PR 3593 added Grok media routes for image generation, image edits, and video generation.
-- APIPool keeps image generation opt-in for existing groups: do not silently
-- flip production groups that an admin may have explicitly kept disabled.
SELECT 1 WHERE FALSE;
