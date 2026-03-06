-- add additional prompt templates for v0
INSERT INTO prompts (id, title, description, template, model, temperature, max_tokens)
SELECT 'summarize_bullets', 'Summarize as Bullets', 'Summarize the text into 5 bullet points.',
  'Summarize the following text into 5 concise bullet points:\n\n{{text}}', 'gpt-5-mini', 0.2, 256
WHERE NOT EXISTS (SELECT 1 FROM prompts WHERE id = 'summarize_bullets');

INSERT INTO prompts (id, title, description, template, model, temperature, max_tokens)
SELECT 'translate_en_es', 'Translate EN→ES', 'Translate English text into Spanish.',
  'Translate the following English text into Spanish:\n\n{{text}}', 'gpt-5-mini', 0.2, 512
WHERE NOT EXISTS (SELECT 1 FROM prompts WHERE id = 'translate_en_es');

INSERT INTO prompts (id, title, description, template, model, temperature, max_tokens)
SELECT 'extract_keywords', 'Extract Keywords', 'Extract up to 10 keywords from the text.',
  'Extract up to 10 keywords (comma separated) from the following text:\n\n{{text}}', 'gpt-5-mini', 0.0, 128
WHERE NOT EXISTS (SELECT 1 FROM prompts WHERE id = 'extract_keywords');

INSERT INTO prompts (id, title, description, template, model, temperature, max_tokens)
SELECT 'rewrite_formal', 'Rewrite (Formal)', 'Rewrite the text in a more formal tone.',
  'Rewrite the following text to be more formal:\n\n{{text}}', 'gpt-5-mini', 0.2, 512
WHERE NOT EXISTS (SELECT 1 FROM prompts WHERE id = 'rewrite_formal');
