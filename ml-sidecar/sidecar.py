import re
import hashlib
import math
from fastapi import FastAPI
from pydantic import BaseModel
from typing import List

app = FastAPI()

# Conversational filler / stopwords that add noise to semantic intent vectors
STOPWORDS = {
    "a", "an", "the", "is", "are", "was", "were", "be", "been", "being",
    "in", "on", "at", "to", "for", "with", "about", "against", "between",
    "into", "through", "during", "before", "after", "above", "below", "from",
    "up", "down", "of", "off", "over", "under", "again", "further", "then",
    "once", "here", "there", "when", "where", "why", "how", "all", "any",
    "both", "each", "few", "more", "most", "other", "some", "such", "no",
    "nor", "not", "only", "own", "same", "so", "than", "too", "very", "can",
    "will", "just", "should", "now", "do", "does", "did", "i", "my", "me",
    "we", "our", "you", "your", "user", "steps", "step", "please", "what",
    "city", "tell", "give", "way", "guide"
}

# Common domain synonyms normalized to canonical tokens
SYNONYMS = {
    "setup": "configure",
    "set": "configure",
    "macbook": "mac",
    "company": "corporate",
    "change": "reset",
    "recover": "reset",
    "passcode": "password",
    "credentials": "password",
}

# Non-entity words that frequently appear capitalized at the start of sentences
SENTENCE_STARTERS = {
    "What", "How", "Steps", "Step", "Where", "When", "Why", "Who", "Which",
    "Please", "Can", "Could", "Would", "Should", "Tell", "Give", "Show",
    "Explain", "List", "Find", "Analyze", "Help", "Is", "Are", "Do", "Does"
}

class TextRequest(BaseModel):
    text: str

class EmbedResponse(BaseModel):
    vector: List[float]

class NERResponse(BaseModel):
    entities: List[str]

@app.post("/v1/embed", response_model=EmbedResponse)
def get_embedding(req: TextRequest):
    text = req.text.lower()
    text = re.sub(r'[^\w\s]', '', text)
    raw_words = text.split()

    # Filter stopwords and map synonyms to extract pure semantic intent
    core_tokens = []
    for w in raw_words:
        if w in STOPWORDS:
            continue
        canonical = SYNONYMS.get(w, w)
        if canonical not in STOPWORDS:
            core_tokens.append(canonical)

    # Fallback to raw words if all words were stopwords
    if not core_tokens:
        core_tokens = raw_words

    vector = [0.0] * 384
    for word in set(core_tokens):
        idx = int(hashlib.sha256(word.encode('utf-8')).hexdigest(), 16) % 384
        vector[idx] += 1.0

    magnitude = math.sqrt(sum(v * v for v in vector))
    if magnitude > 0:
        vector = [v / magnitude for v in vector]

    return EmbedResponse(vector=vector)

@app.post("/v1/ner", response_model=NERResponse)
def get_entities(req: TextRequest):
    text = req.text
    # Extract capitalized proper nouns and PII tokens (e.g., "Japan", "France", "EMAIL_1")
    words = re.findall(r'\b[A-Z][a-zA-Z0-9_]+\b', text)

    # Filter out common sentence-starter words and single-letter pronouns ("I", "A")
    entities = [
        w.lower() for w in words
        if w not in SENTENCE_STARTERS and len(w) > 1
    ]

    unique_entities = sorted(list(set(entities)))
    return NERResponse(entities=unique_entities)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
