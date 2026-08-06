import openai

# Missing max_tokens
response1 = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)

# Has max_tokens
response2 = openai.ChatCompletion.create(
    model="gpt-3.5-turbo",
    max_tokens=100,
    messages=[{"role": "user", "content": "Hi!"}]
)
