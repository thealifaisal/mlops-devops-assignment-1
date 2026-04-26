import json
import os
import urllib.request
import urllib.error


def lambda_handler(event, context):
    api_key = os.environ.get("OPENAI_API_KEY", "")
    if not api_key:
        return {
            "text": "",
            "usage": {"inputTokens": 0, "outputTokens": 0},
            "error": "OPENAI_API_KEY is not set in Lambda environment"
        }

    prompt      = event.get("prompt", "")
    model       = event.get("model", "gpt-4o-mini")
    temperature = event.get("temperature", 0.2)

    payload = json.dumps({
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": temperature
    }).encode("utf-8")

    req = urllib.request.Request(
        "https://api.openai.com/v1/chat/completions",
        data=payload,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json"
        },
        method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            body = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8", errors="replace")
        return {
            "text": "",
            "usage": {"inputTokens": 0, "outputTokens": 0},
            "error": f"OpenAI HTTP {e.code}: {error_body}"
        }
    except Exception as e:
        return {
            "text": "",
            "usage": {"inputTokens": 0, "outputTokens": 0},
            "error": str(e)
        }

    text = body["choices"][0]["message"]["content"]
    usage = body.get("usage", {})

    return {
        "text": text,
        "usage": {
            "inputTokens":  usage.get("prompt_tokens", 0),
            "outputTokens": usage.get("completion_tokens", 0)
        }
    }
