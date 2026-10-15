import re
import hashlib
import math
from fastapi import FastAPI
from pydantic import BaseModel
from typing import List

app = FastAPI()

class TextRequest(BaseModel):
    text: str

class EmbedResponse(BaseModel):
    vector: List[float]

class NERResponse(BaseModel):
    entities: List[str]

@app.post("/v1/embed", response_model=EmbedResponse)
def get_embedding(req: TextRequest):
    # Bag-of-Words Mock Embedding (No heavy dependencies!)
    text = req.text.lower()
    # Remove punctuation
    text = re.sub(r'[^\w\s]', '', text)
    words = text.split()
    
    # Initialize 384-dimensional vector
    vector = [0.0] * 384
    
    # Hash each word to an index and increment
    for word in set(words): # Use set to avoid double counting same word
        idx = int(hashlib.sha256(word.encode('utf-8')).hexdigest(), 16) % 384
        vector[idx] += 1.0
        
    # Normalize the vector (so dot product equals cosine similarity)
    magnitude = math.sqrt(sum(v*v for v in vector))
    if magnitude > 0:
        vector = [v / magnitude for v in vector]
        
    return EmbedResponse(vector=vector)

@app.post("/v1/ner", response_model=NERResponse)
def get_entities(req: TextRequest):
    text = req.text
    entities = []
    
    # Simple Mock: Extract Capitalized words (e.g. "Japan")
    words = re.findall(r'\b[A-Z][a-z]+\b', text)
    entities.extend(words)
    
    unique_entities = sorted(list(set(entities)))
    return NERResponse(entities=unique_entities)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
