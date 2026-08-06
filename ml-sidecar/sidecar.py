import re
import hashlib
import random
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
    # Deterministic hash of the input string
    seed_str = req.text.strip().lower()
    hash_obj = hashlib.sha256(seed_str.encode('utf-8'))
    seed_int = int(hash_obj.hexdigest(), 16)
    
    # Use the hash as a seed for the random number generator
    random.seed(seed_int)
    
    # Generate 384 pseudo-random floats between -1.0 and 1.0
    vector = [random.uniform(-1.0, 1.0) for _ in range(384)]
    
    return EmbedResponse(vector=vector)

@app.post("/v1/ner", response_model=NERResponse)
def get_entities(req: TextRequest):
    text = req.text
    entities = []
    
    # 1. Emails
    email_pattern = r'[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'
    emails = re.findall(email_pattern, text)
    entities.extend(emails)
    
    # 2. Dates (YYYY) - Simple 4 digit year
    year_pattern = r'\b(19|20)\d{2}\b'
    years = [m.group(0) for m in re.finditer(year_pattern, text)]
    entities.extend(years)
    
    # 3. Currency / Numbers (e.g. $100, 500)
    # Simple regex for numbers possibly preceded by a currency symbol
    money_pattern = r'[\$£€]?\b\d+(?:\.\d{2})?\b'
    money = re.findall(money_pattern, text)
    # Exclude years from money if they accidentally matched
    money = [m for m in money if m not in years]
    entities.extend(money)
    
    # Deduplicate and sort
    unique_entities = sorted(list(set(entities)))
    
    return NERResponse(entities=unique_entities)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
